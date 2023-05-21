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
)

const (
	defaultPort = 50053
)

type GrpcAdapter struct {
	tcpListener       net.Listener
	server            *grpc.Server
	healthcheckServer *health.Server
}

func CreateGrpcAdapter() *GrpcAdapter {
	var (
		g   = &GrpcAdapter{}
		err error
	)

	port := utils.GetenvOr("ANSIBLER_PORT", fmt.Sprint(defaultPort))
	tcpBindingAddress := net.JoinHostPort("0.0.0.0", port)
	g.tcpListener, err = net.Listen("tcp", tcpBindingAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s : %v", tcpBindingAddress, err)
	}
	log.Info().Msgf("Ansibler microservice is listening on %s", tcpBindingAddress)

	g.server = grpc.NewServer()
	pb.RegisterAnsiblerServiceServer(g.server, &AnsiblerGrpcService{})

	// Add healthcheck service to gRPC
	g.healthcheckServer = health.NewServer()
	// Ansibler microservice does not have any custom healthcheck functions,
	// thus always serving.
	g.healthcheckServer.SetServingStatus("ansibler-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	g.healthcheckServer.SetServingStatus("ansibler-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.healthcheckServer)

	return g
}

func (g *GrpcAdapter) Serve() error {
	// g.Serve() will create a service goroutine for each incoming connection
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("ansibler microservice failed to serve gRPC request: %w", err)
	}

	log.Info().Msg("Finished listening for incoming gRPC requests")
	return nil
}

func (g *GrpcAdapter) Stop() {
	log.Info().Msg("Gracefully shutting down gRPC server")

	g.server.GracefulStop()
	g.healthcheckServer.Shutdown()
}
