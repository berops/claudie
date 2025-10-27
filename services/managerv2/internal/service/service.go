package service

import (
	"context"
	"fmt"
	"net"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/managerv2/internal/store"
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
)

var _ pb.ManagerV2ServiceServer = (*Service)(nil)

type Service struct {
	pb.UnimplementedManagerV2ServiceServer

	lis net.Listener

	server            *grpc.Server
	healthCheckServer *health.Server

	nats *natsutils.Client

	store store.Store
}

func New(ctx context.Context, opts ...grpc.ServerOption) (*Service, error) {
	natsClient, err := natsutils.NewClientWithJetStream(envs.NatsClusterURL, envs.NatsClusterSize)
	if err != nil {
		return nil, err
	}

	if err := natsClient.JetStreamWorkQueue(ctx, envs.NatsClusterJetstreamName); err != nil {
		natsClient.Close()
		return nil, fmt.Errorf("failed to create/update %q queue", envs.NatsClusterJetstreamName)
	}

	log.Info().Msgf("jetstream %q successfully initialized", envs.NatsClusterJetstreamName)

	listeningAddress := net.JoinHostPort("0.0.0.0", fmt.Sprint(Port))

	lcfg := net.ListenConfig{}
	lis, err := lcfg.Listen(ctx, "tcp", listeningAddress)
	if err != nil {
		natsClient.Close()
		return nil, fmt.Errorf("failed to bind tcp socket for address: %q: %w", listeningAddress, err)
	}

	log.Info().Msgf("manager microservice bound to %s", listeningAddress)

	grpcServer := grpcutils.NewGRPCServer(opts...)

	mongo, err := store.NewMongoClient(ctx, envs.DatabaseURL)
	if err != nil {
		grpcServer.Stop()
		natsClient.Close()
		lis.Close()
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	if err := mongo.Init(); err != nil {
		grpcServer.Stop()
		natsClient.Close()
		lis.Close()
		mongo.Close()
		return nil, fmt.Errorf("failed to init mongo database: %w", err)
	}

	// Add health-check service to gRPC
	healthCheckServer := health.NewServer()

	healthCheckServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthCheckServer)

	s := &Service{
		lis:               lis,
		server:            grpcServer,
		healthCheckServer: healthCheckServer,
		nats:              natsClient,
		store:             mongo,
	}

	pb.RegisterManagerV2ServiceServer(s.server, s)

	return s, nil
}

// Serve will create a service goroutine for each connection
func (s *Service) Serve() error {
	if err := s.server.Serve(s.lis); err != nil {
		return fmt.Errorf("manager microservice grpc server failed to serve: %w", err)
	}

	log.Info().Msgf("Finished listening for incoming gRPC connections")
	return nil
}

// Stop will gracefully shutdown the gRPC server and the healthcheck server
func (s *Service) Stop() error {
	s.nats.Close()
	s.server.GracefulStop()
	s.healthCheckServer.Shutdown()
	return s.store.Close()
}

func (s *Service) PerformHealthCheckAndUpdateStatus() {
	if err := s.store.HealthCheck(); err != nil {
		s.healthCheckServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}
	s.healthCheckServer.SetServingStatus(HealthCheckName, grpc_health_v1.HealthCheckResponse_SERVING)
}
