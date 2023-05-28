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
	"github.com/berops/claudie/services/kuber/server/domain/usecases"
)

const defaultPort = 50057

type GrpcAdapter struct {
	tcpListener       net.Listener
	server            *grpc.Server
	healthcheckServer *health.Server
}

func (g *GrpcAdapter) Init(usecases *usecases.Usecases) {
	port := utils.GetenvOr("KUBER_PORT", fmt.Sprint(defaultPort))

	var (
		err error

		bindingAddress = net.JoinHostPort("0.0.0.0", port)
	)
	g.tcpListener, err = net.Listen("tcp", bindingAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to listen %v", err)
	}
	log.Info().Msgf("Kuber microservice is listening: %s", bindingAddress)

	g.server = grpc.NewServer()
	pb.RegisterKuberServiceServer(g.server, &KuberGrpcService{usecases: usecases})

	// Add healthcheck service to the gRPC server
	g.healthcheckServer = health.NewServer()
	// Kuber does not have any custom health check functions, thus always serving.
	g.healthcheckServer.SetServingStatus("kuber-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	g.healthcheckServer.SetServingStatus("kuber-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.healthcheckServer)
}

func (g *GrpcAdapter) Serve() error {
	// g.server.Serve() will create a goroutine for each incoming gRPC connection.
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("kuber microservice failed to serve: %w", err)
	}

	log.Info().Msg("Finished listening for incoming gRPC connections")
	return nil
}

func (g *GrpcAdapter) Stop() {
	g.server.Stop()
	g.healthcheckServer.Shutdown()
}
