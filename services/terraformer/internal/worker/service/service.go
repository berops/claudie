package service

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	// Port on which the health check service will be listening on.
	Port = envs.GetOrDefaultInt("TERRAFORMER_PORT", 50052)

	// SpawnProcessLimit is the number of processes concurrently executing tofu.
	SpawnProcessLimit = envs.GetOrDefaultInt("TERRAFORMER_CONCURRENT_CLUSTERS", 7)

	// Durable Name of this service.
	DurableName = envs.GetOrDefault("TERRAFORMER_DURABLE_NAME", "terraformer")

	// Name used for health checking via the grpc health check.
	HealthCheckReadinessName = envs.GetOrDefault("TERRAFORMER_HEALTHCHECK_READINESS_SERVICE_NAME", "terraformer-readiness")
	HealthCheckLivenessName  = envs.GetOrDefault("TERRAFORMER_HEALTHCHECK_LIVENESS_SERVICE_NAME", "terraformer-liveness")

	// Ack wait time in minutes for processing incoming NATS messages.
	AckWait = time.Duration(envs.GetOrDefaultInt("TERRAFORMER_ACK_WAIT_TIME", 8)) * time.Minute
)

type grpcServer struct {
	tcpListener  net.Listener
	server       *grpc.Server
	healthServer *health.Server
}

type natsConsumer struct {
	natsclient  *natsutils.Client
	consumer    jetstream.Consumer
	consumerCtx jetstream.ConsumeContext
	inFlight    sync.WaitGroup
}

type Service struct {
	stateStorage      store.S3StateStorage
	spawnProcessLimit *semaphore.Weighted

	gserver  *grpcServer
	consumer *natsConsumer

	done chan struct{}
}

func New(ctx context.Context, opts ...grpc.ServerOption) (*Service, error) {
	listenerAddress := net.JoinHostPort("0.0.0.0", fmt.Sprintf("%v", Port))
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(ctx, "tcp", listenerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on address %s: %w", listenerAddress, err)
	}

	// The client is created with default options which include
	// an infinite number of reconnects, thus until the service
	// itself is not killed, it should always try to stay up
	// and process incoming messages.
	client, err := natsutils.NewClientWithJetStream(envs.NatsClusterURL, envs.NatsClusterSize)
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to connect to nats cluster at %s with size %v: %w", envs.NatsClusterURL, envs.NatsClusterSize, err)
	}

	consumer, err := client.JSWorkQueueConsumer(
		ctx,
		DurableName,
		envs.NatsClusterJetstreamName,
		AckWait,
		natsutils.TerraformerRequests,
	)
	if err != nil {
		listener.Close()
		client.Close()
		return nil, fmt.Errorf("failed to create consumer for jetstream %s: %w", envs.NatsClusterJetstreamName, err)
	}

	grpcserver := grpcutils.NewGRPCServer(opts...)
	healthserver := health.NewServer()

	healthserver.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	healthserver.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcserver, healthserver)

	s3 := store.CreateS3Adapter()
	spawnLimit := semaphore.NewWeighted(int64(SpawnProcessLimit))

	gserver := grpcServer{
		tcpListener:  listener,
		server:       grpcserver,
		healthServer: healthserver,
	}

	natsconsumer := natsConsumer{
		natsclient: client,
		consumer:   consumer,
	}

	s := Service{
		stateStorage:      s3,
		spawnProcessLimit: spawnLimit,
		gserver:           &gserver,
		consumer:          &natsconsumer,
		done:              make(chan struct{}),
	}

	consumeOptions := [...]jetstream.PullConsumeOpt{
		jetstream.ConsumeErrHandler(errHandler),
		// The consumer will by default buffer messages behind the scenes and if the messages are not
		// acknowledged, even if they're buffered, within the specified ack timeout they will be re-send,
		// thus we always keep a maximum of 1 message to be buffered. To then handle multiple msgs at once
		// we process each message in a go-routine.
		jetstream.PullMaxMessages(1),
	}

	if s.consumer.consumerCtx, err = consumer.Consume(s.Handler, consumeOptions[:]...); err != nil {
		s.Stop()
		return nil, fmt.Errorf("failed to start consumer handler: %w", err)
	}

	return &s, nil
}

func (s *Service) ServeHealthChecks() error {
	if err := s.gserver.server.Serve(s.gserver.tcpListener); err != nil {
		return fmt.Errorf("failed to serve health checks: %w", err)
	}
	log.Info().Msg("Finished listening for incoming health checks")
	return nil
}

func (s *Service) Stop() {
	log.Info().Msg("Gracefully shutting down serivce")

	// unsubscribe and discard any buffered messages in NATS.
	s.consumer.consumerCtx.Stop()

	// signal we are closing to all spawned go-routines.
	close(s.done)

	// wait for current in-filght messages to finish.
	s.consumer.inFlight.Wait()

	// wait for the consumer to close.
	<-s.consumer.consumerCtx.Closed()

	// we are no longer processing any messages, close all other connections.
	s.consumer.natsclient.Close()
	s.gserver.server.GracefulStop()
	s.gserver.healthServer.Shutdown()
	s.gserver.tcpListener.Close()
}

func (s *Service) PerformHealthCheckAndUpdateStatus() {
	if err := s.stateStorage.HealthCheck(); err != nil {
		s.gserver.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.gserver.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}

	if status := s.consumer.natsclient.Conn().Status(); status != nats.CONNECTED {
		err := fmt.Errorf("nats connection status is %s", status.String())
		s.gserver.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.gserver.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}

	s.gserver.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_SERVING)
	s.gserver.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_SERVING)
}
