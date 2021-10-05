package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/rs/zerolog/log"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct{}

func (*server) BuildInfrastructure(ctx context.Context, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	log.Info().Msgf("BuildInfrastructure function was invoked with config %s", req.GetConfig().GetName())
	config := req.GetConfig()
	err := buildInfrastructure(config)
	if err != nil {
		return nil, fmt.Errorf("template generator failed: %v", err)
	}
	log.Info().Msg("Infrastructure was successfully generated")
	return &pb.BuildInfrastructureResponse{Config: config}, nil
}

func (*server) DestroyInfrastructure(ctx context.Context, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	fmt.Println("DestroyInfrastructure function was invoked with config:", req.GetConfig().GetName())
	config := req.GetConfig()
	err := destroyInfrastructure(config.GetCurrentState())
	if err != nil {
		return nil, fmt.Errorf("error while destroying the infrastructure: %v", err)
	}

	return &pb.DestroyInfrastructureResponse{Config: config}, nil
}

func main() {
	// intialize logging framework
	utils.InitLog("terraformer")

	// Set the context-box port
	terraformerPort := os.Getenv("TERRAFORMER_PORT")
	if terraformerPort == "" {
		terraformerPort = "50052" // Default value
	}

	// Start Terraformer Service
	trfAddr := "0.0.0.0:" + terraformerPort
	lis, err := net.Listen("tcp", trfAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s", err)
	}
	log.Info().Msgf("Terraformer service is listening on: %s", trfAddr)

	s := grpc.NewServer()
	pb.RegisterTerraformerServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker("50052", "TERRAFORMER_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	{
		g.Go(func() error {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)
			defer signal.Stop(ch)
			<-ch

			signal.Stop(ch)
			s.GracefulStop()

			return errors.New("interrupt signal")
		})
	}
	{
		g.Go(func() error {
			// s.Serve() will create a service goroutine for each connection
			if err := s.Serve(lis); err != nil {
				return fmt.Errorf("failed to serve: %v", err)
			}
			return nil
		})
	}

	log.Info().Msgf("Stopping Terraformer: %v", g.Wait())
}
