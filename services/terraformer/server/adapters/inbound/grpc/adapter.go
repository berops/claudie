package grpc

import (
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/usecases"
)

const (
	defaultTerraformerPort = 50052
)

type GrpcAdapter struct {
	tcpListener  net.Listener
	server       *grpc.Server
	HealthServer *health.Server
}

// Init sets up the GrpcAdapter by creating the underlying tcpListener, gRPC server and
// gRPC health check server.
func (g *GrpcAdapter) Init(usecases *usecases.Usecases) {
	port := utils.GetEnvDefault("TERRAFORMER_PORT", fmt.Sprint(defaultTerraformerPort))

	var err error

	listeningAddress := net.JoinHostPort("0.0.0.0", port)
	g.tcpListener, err = net.Listen("tcp", listeningAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %v", err)
	}
	log.Info().Msgf("Terraformer service is listening on: %s", listeningAddress)

	g.server = grpc.NewServer()
	pb.RegisterTerraformerServiceServer(g.server, &TerraformerGrpcService{usecases: usecases})

	// Add health service to gRPC
	g.HealthServer = health.NewServer()
	// Set liveness to SERVING
	g.HealthServer.SetServingStatus("terraformer-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	// Set readiness to NOT_SERVING, as it will be changed later.
	g.HealthServer.SetServingStatus("terraformer-readiness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.HealthServer)
}

// Start makes the gRPC server start listening for incoming gRPC requests.
func (g *GrpcAdapter) Start() error {
	// Process each gRPC request in a separate thread.
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("terraformer failed to serve: %w", err)
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
