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
	Port = envs.GetOrDefaultInt("KUBER_PORT", 50057)

	// SpawnProcessLimit is the number of processes concurrently executing kubectl commands.
	SpawnProcessLimit = envs.GetOrDefaultInt("KUBER_CONCURRENT_CALLS", 90)

	// WorkersLimit is the maximum amount of workers a single work group can have.
	WorkersLimit = envs.GetOrDefaultInt("KUBER_CONCURRENT_WORKERS", 30)

	// Durable Name of this service.
	DurableName = envs.GetOrDefault("KUBER_DURABLE_NAME", "kuber")

	// Name used for health checking via the grpc health check.
	HealthCheckName = envs.GetOrDefault("KUBER_HEALTHCHECK_SERVICE_NAME", "kuber-readiness")

	// Ack wait time in minutes for processing incoming NATS messages.
	AckWait = time.Duration(envs.GetOrDefaultInt("KUBER_ACK_WAIT_TIME", 10)) * time.Minute
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
	spawnProcessLimit *semaphore.Weighted
	workersLimit      int

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
		natsutils.KuberRequests,
	)
	if err != nil {
		listener.Close()
		client.Close()
		return nil, fmt.Errorf("failed to create consumer for jetstream %s: %w", envs.NatsClusterJetstreamName, err)
	}

	grpcserver := grpcutils.NewGRPCServer(opts...)
	healthserver := health.NewServer()

	healthserver.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcserver, healthserver)

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
		spawnProcessLimit: spawnLimit,
		workersLimit:      WorkersLimit,
		gserver:           &gserver,
		consumer:          &natsconsumer,
		done:              make(chan struct{}),
	}

	consumeOptions := [...]jetstream.PullConsumeOpt{
		jetstream.ConsumeErrHandler(errHandler),
		// The consumer will by default buffer messages behind the scenes and if the messages are not
		// acknowledged, even if they're buffered, within the specified ack timeout they will be re-send,
		// thus we always keep a maximum of 1 message to be buffered. To then handle multiple msgs at once
		// we process each message in each go-routine.
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
	if status := s.consumer.natsclient.Conn().Status(); status != nats.CONNECTED {
		err := fmt.Errorf("nats connection status is %s", status.String())
		s.gserver.healthServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}

	s.gserver.healthServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_SERVING)
}
