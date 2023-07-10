package grpc

import (
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/claudie-operator/server/domain/usecases"
)

const (
	defaultOperatorPort = 50058
)

type GrpcAdapter struct {
	tcpListener net.Listener
	server      *grpc.Server
}

// Init will create the underlying gRPC server and the gRPC healthcheck server
func (g *GrpcAdapter) Init(usecases *usecases.Usecases) {
	port := utils.GetEnvDefault("OPERATOR_PORT", fmt.Sprint(defaultOperatorPort))
	listeningAddress := net.JoinHostPort("0.0.0.0", port)

	tcpListener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to start Grpc server for claudie-operator at %s: %v", listeningAddress, err)
	}
	g.tcpListener = tcpListener

	log.Info().Msgf("Claudie-operator bound to %s", listeningAddress)

	g.server = utils.NewGRPCServer()
	pb.RegisterOperatorServiceServer(g.server, &OperatorGrpcService{usecases: usecases})
}

// Serve will create a service goroutine for each connection
func (g *GrpcAdapter) Serve() error {
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("claudie-operator grpc server failed to serve: %w", err)
	}

	log.Info().Msgf("Finished listening for incoming gRPC connections")
	return nil
}

// Stop will gracefully shutdown the gRPC server
func (g *GrpcAdapter) Stop() {
	g.server.GracefulStop()
}
