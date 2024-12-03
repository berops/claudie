package service

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"
)

const defaultManagerPort = 50055

type GRPC struct {
	pb.UnimplementedManagerServiceServer

	tcpListener net.Listener

	server            *grpc.Server
	HealthCheckServer *health.Server

	Store store.Store
}

func NewGRPC(ctx context.Context, opts ...grpc.ServerOption) (*GRPC, error) {
	g := new(GRPC)

	port := envs.GetOrDefault("MANAGER_PORT", fmt.Sprint(defaultManagerPort))
	listeningAddress := net.JoinHostPort("0.0.0.0", port)

	tcpListener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to bind tcp socket for address: %q: %w", listeningAddress, err)
	}
	g.tcpListener = tcpListener

	log.Info().Msgf("manager microservice bound to %s", listeningAddress)

	g.server = grpcutils.NewGRPCServer(opts...)
	pb.RegisterManagerServiceServer(g.server, g)

	// Add health-check service to gRPC
	g.HealthCheckServer = health.NewServer()
	// Manager does not have any custom health check functions, thus always serving.
	g.HealthCheckServer.SetServingStatus("manager-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.HealthCheckServer)

	mongo, err := store.NewMongoClient(ctx, envs.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	if err := mongo.Init(); err != nil {
		return nil, fmt.Errorf("failed to init mongo database: %w", err)
	}

	g.Store = mongo

	return g, nil
}

// Serve will create a service goroutine for each connection
func (g *GRPC) Serve() error {
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("manager microservice grpc server failed to serve: %w", err)
	}

	log.Info().Msgf("Finished listening for incoming gRPC connections")
	return nil
}

// Stop will gracefully shutdown the gRPC server and the healthcheck server
func (g *GRPC) Stop() error {
	g.server.GracefulStop()
	g.HealthCheckServer.Shutdown()
	return g.Store.Close()
}
