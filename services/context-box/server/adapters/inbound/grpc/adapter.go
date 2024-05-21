package grpc

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"net"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/domain/usecases"
)

const (
	defaultContextBoxPort = 50055
)

type GrpcAdapter struct {
	tcpListener       net.Listener
	server            *grpc.Server
	HealthCheckServer *health.Server
}

// Init will create the underlying gRPC server and the gRPC healthcheck server
func (g *GrpcAdapter) Init(usecases *usecases.Usecases, opts ...grpc.ServerOption) {
	port := utils.GetEnvDefault("CONTEXT_BOX_PORT", fmt.Sprint(defaultContextBoxPort))
	listeningAddress := net.JoinHostPort("0.0.0.0", port)

	tcpListener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to start Grpc server for context-box microservice at %s: %v", listeningAddress, err)
	}
	g.tcpListener = tcpListener

	log.Info().Msgf("context-box microservice bound to %s", listeningAddress)

	g.server = utils.NewGRPCServer(opts...)
	pb.RegisterContextBoxServiceServer(g.server, &ContextBoxGrpcService{usecases: usecases})

	// Add health-check service to gRPC
	g.HealthCheckServer = health.NewServer()
	// Context-box does not have any custom health check functions, thus always serving.
	g.HealthCheckServer.SetServingStatus("context-box-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.HealthCheckServer)
}

// Serve will create a service goroutine for each connection
func (g *GrpcAdapter) Serve() error {
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("context-box microservice grpc server failed to serve: %w", err)
	}

	log.Info().Msgf("Finished listening for incoming gRPC connections")
	return nil
}

// Stop will gracefully shutdown the gRPC server and the healthcheck server
func (g *GrpcAdapter) Stop() {
	g.server.GracefulStop()
	g.HealthCheckServer.Shutdown()
}
