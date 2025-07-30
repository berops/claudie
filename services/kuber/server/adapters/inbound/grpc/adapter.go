package grpc

import (
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/usecases"
)

const (
	defaultKuberPort = 50057
)

type GrpcAdapter struct {
	tcpListener  net.Listener
	server       *grpc.Server
	HealthServer *health.Server
}

// Init sets up the GrpcAdapter by creating the underlying tcpListener, gRPC server and
// gRPC health check server.
func (g *GrpcAdapter) Init(usecases *usecases.Usecases, opts ...grpc.ServerOption) {
	port := envs.GetOrDefault("KUBER_PORT", fmt.Sprint(defaultKuberPort))

	var err error

	listeningAddress := net.JoinHostPort("0.0.0.0", port)
	//nolint
	g.tcpListener, err = net.Listen("tcp", listeningAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %v", err)
	}
	log.Info().Msgf("Kuber service is listening on: %s", listeningAddress)

	g.server = grpcutils.NewGRPCServer(opts...)
	pb.RegisterKuberServiceServer(g.server, &KuberGrpcService{usecases: usecases})

	// Add health service to gRPC
	g.HealthServer = health.NewServer()
	// Kuber does not have any custom health check functions, thus always serving.
	g.HealthServer.SetServingStatus("kuber-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	g.HealthServer.SetServingStatus("kuber-readiness", grpc_health_v1.HealthCheckResponse_SERVING)

	grpc_health_v1.RegisterHealthServer(g.server, g.HealthServer)
}

// Serve makes the gRPC server start listening for incoming gRPC requests.
func (g *GrpcAdapter) Serve() error {
	// Process each gRPC request in a separate thread.
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("kuber failed to serve: %w", err)
	}

	log.Info().Msg("Finished listening for incoming connections")
	return nil
}

// Stop gracefully shuts down the underlying gRPC server and gRCP health-check server.
func (g *GrpcAdapter) Stop() {
	log.Info().Msg("Gracefully shutting down gRPC server")

	g.server.GracefulStop()
	g.HealthServer.Shutdown()
}
