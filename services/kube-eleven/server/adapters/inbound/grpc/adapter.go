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
	"github.com/berops/claudie/services/kube-eleven/server/domain/usecases"
)

const (
	defaultPort = 50054
)

type GrpcAdapter struct {
	tcpListener       net.Listener
	server            *grpc.Server
	healthcheckServer *health.Server
}

func CreateGrpcAdapter(usecases *usecases.Usecases) *GrpcAdapter {
	var (
		g   = &GrpcAdapter{}
		err error
	)

	port := utils.GetEnvDefault("KUBE_ELEVEN_PORT", fmt.Sprint(defaultPort))
	bindingAddress := net.JoinHostPort("0.0.0.0", port)
	g.tcpListener, err = net.Listen("tcp", bindingAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to bind to %s : %v", bindingAddress, err)
	}
	log.Info().Msgf("Kube-eleven microservice is listening on %s", bindingAddress)

	g.server = grpc.NewServer()
	pb.RegisterKubeElevenServiceServer(g.server, &KubeElevenGrpcService{usecases: usecases})

	// Add healthcheck service to the gRPC server
	g.healthcheckServer = health.NewServer()
	// Kube-eleven does not have any custom healthcheck functions,
	// thus always serving.
	g.healthcheckServer.SetServingStatus("kube-eleven-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	g.healthcheckServer.SetServingStatus("kube-eleven-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.healthcheckServer)

	return g
}

func (g *GrpcAdapter) Serve() error {
	// g.Serve() will create a service goroutine for each connection
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("kube-eleven failed to serve gRPC request: %w", err)
	}

	log.Info().Msg("Finished listening for incoming gRPC connections")
	return nil
}

func (g *GrpcAdapter) Stop() {
	log.Info().Msg("Gracefully shutting down gRPC server")
	g.server.GracefulStop()
	g.healthcheckServer.Shutdown()
}
