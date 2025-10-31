package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/managerv2/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	// Port on which the grpc server will be listening on.
	Port = envs.GetOrDefaultInt("MANAGER_PORT", 50055)

	// Durable name of this service.
	DurableName = envs.GetOrDefault("MANAGER_DURABLE_NAME", "manager")

	// Name used for health checking via the grpc health check.
	HealthCheckName = envs.GetOrDefault("MANAGER_HEALTHCHECK_SERVICE_NAME", "manager-readiness")

	// Ack wait time in minutes for processing incoming NATS messages.
	AckWait = time.Duration(envs.GetOrDefaultInt("MANAGER_ACK_WAIT_TIME", 10)) * time.Minute
)

var _ pb.ManagerV2ServiceServer = (*Service)(nil)

type grpcServer struct {
	tcpListener  net.Listener
	server       *grpc.Server
	healthServer *health.Server
}

type natsClient struct {
	client      *natsutils.Client
	consumer    jetstream.Consumer
	consumerCtx jetstream.ConsumeContext
	inFlight    sync.WaitGroup
}

type Service struct {
	pb.UnimplementedManagerV2ServiceServer

	store store.Store

	server *grpcServer
	nts    *natsClient

	done chan struct{}
}

func New(ctx context.Context, opts ...grpc.ServerOption) (*Service, error) {
	client, err := natsutils.NewClientWithJetStream(envs.NatsClusterURL, envs.NatsClusterSize)
	if err != nil {
		return nil, err
	}

	if err := client.JetStreamWorkQueue(ctx, envs.NatsClusterJetstreamName); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create/update %q queue", envs.NatsClusterJetstreamName)
	}

	consumer, err := client.JSWorkQueueConsumer(
		ctx,
		DurableName,
		envs.NatsClusterJetstreamName,
		AckWait,
		natsutils.AnsiblerResponse,
		natsutils.KuberResponse,
		natsutils.KubeElevenResponse,
		natsutils.TerraformerResponse,
	)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create consumer for jetstream %s: %w", envs.NatsClusterJetstreamName, err)
	}

	log.Info().Msgf("jetstream %q successfully initialized", envs.NatsClusterJetstreamName)

	listeningAddress := net.JoinHostPort("0.0.0.0", fmt.Sprint(Port))

	lcfg := net.ListenConfig{}
	lis, err := lcfg.Listen(ctx, "tcp", listeningAddress)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to bind tcp socket for address: %q: %w", listeningAddress, err)
	}

	log.Info().Msgf("manager microservice bound to %s", listeningAddress)

	mongo, err := store.NewMongoClient(ctx, envs.DatabaseURL)
	if err != nil {
		client.Close()
		lis.Close()
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	if err := mongo.Init(); err != nil {
		client.Close()
		lis.Close()
		mongo.Close()
		return nil, fmt.Errorf("failed to init mongo database: %w", err)
	}

	grpcserver := grpcutils.NewGRPCServer(opts...)
	healthserver := health.NewServer()

	healthserver.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcserver, healthserver)

	gserver := grpcServer{
		tcpListener:  lis,
		server:       grpcserver,
		healthServer: healthserver,
	}

	natsconsumer := natsClient{
		client:   client,
		consumer: consumer,
	}

	s := &Service{
		store:  mongo,
		server: &gserver,
		nts:    &natsconsumer,
		done:   make(chan struct{}),
	}

	consumeOptions := [...]jetstream.PullConsumeOpt{
		jetstream.ConsumeErrHandler(errHandler),
		// The consumer will by default buffer messages behind the scenes and if the messages are not
		// acknowledged, even if they're buffered, within the specified ack timeout they will be re-send,
		// thus we always keep a maximum of 1 message to be buffered. To then handle multiple msgs at once
		// we process each message in each go-routine.
		jetstream.PullMaxMessages(1),
	}

	if s.nts.consumerCtx, err = consumer.Consume(s.Handler, consumeOptions[:]...); err != nil {
		s.Stop()
		return nil, fmt.Errorf("failed to start nats consumer handler: %w", err)
	}

	pb.RegisterManagerV2ServiceServer(s.server.server, s)

	return s, nil
}

// Serve will create a service goroutine for each connection
func (s *Service) Serve() error {
	if err := s.server.server.Serve(s.server.tcpListener); err != nil {
		return fmt.Errorf("manager microservice grpc server failed to serve: %w", err)
	}

	log.Info().Msgf("Finished listening for incoming gRPC connections")
	return nil
}

// Stop will gracefully shutdown the gRPC server and the healthcheck server
func (s *Service) Stop() error {
	log.Info().Msg("Gracefully shutting down service")

	// unsubscribe and discard any buffered messages in NATS.
	s.nts.consumerCtx.Stop()

	// signal we are closing to all spawned go-routine.
	close(s.done)

	// wait for current in-flight messages to finish.
	s.nts.inFlight.Wait()

	// wait for the consumer to close.
	<-s.nts.consumerCtx.Closed()

	s.nts.client.Close()
	s.server.server.GracefulStop()
	s.server.healthServer.Shutdown()

	err := s.server.tcpListener.Close()
	if errc := s.store.Close(); errc != nil {
		err = errors.Join(err, errc)
	}

	return err
}

func (s *Service) PerformHealthCheckAndUpdateStatus() {
	if err := s.store.HealthCheck(); err != nil {
		s.server.healthServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}
	s.server.healthServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_SERVING)
}
