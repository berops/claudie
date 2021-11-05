package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const defaultTerraformerPort = 50052

type server struct {
	pb.UnimplementedTerraformerServiceServer
}

func (*server) BuildInfrastructure(ctx context.Context, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	log.Info().Msgf("BuildInfrastructure function was invoked with config %s", req.GetConfig().GetName())
	config := req.GetConfig()
	err := buildInfrastructure(config)
	if err != nil {
		config.ErrorMessage = err.Error()
		return &pb.BuildInfrastructureResponse{Config: config}, fmt.Errorf("template generator failed: %v", err)
	}
	log.Info().Msg("Infrastructure was successfully generated")
	config.ErrorMessage = ""
	for _, cluster := range config.DesiredState.Clusters {
		fmt.Printf("jaskeerat cluster: %v\n", cluster)
	}
	return &pb.BuildInfrastructureResponse{Config: config}, nil
}

func (*server) DestroyInfrastructure(ctx context.Context, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	fmt.Println("DestroyInfrastructure function was invoked with config:", req.GetConfig().GetName())
	config := req.GetConfig()
	err := destroyInfrastructure(config)
	if err != nil {
		config.ErrorMessage = err.Error()
		return &pb.DestroyInfrastructureResponse{Config: config}, fmt.Errorf("error while destroying the infrastructure: %v", err)
	}
	config.ErrorMessage = ""
	return &pb.DestroyInfrastructureResponse{Config: config}, nil
}

func main() {
	// initialize logger
	utils.InitLog("terraformer", "GOLANG_LOG")

	// Set the context-box port
	terraformerPort := utils.GetenvOr("TERRAFORMER_PORT", fmt.Sprint(defaultTerraformerPort))

	// Start Terraformer Service
	trfAddr := net.JoinHostPort("0.0.0.0", terraformerPort)
	lis, err := net.Listen("tcp", trfAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %v", err)
	}
	log.Info().Msgf("Terraformer service is listening on: %s", trfAddr)

	s := grpc.NewServer()
	pb.RegisterTerraformerServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker(terraformerPort, "TERRAFORMER_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch

		signal.Stop(ch)
		s.GracefulStop()

		return errors.New("terraformer interrupt signal")
	})

	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("terraformer failed to serve: %v", err)
		}
		return nil
	})

	log.Info().Msgf("Stopping Terraformer: %v", g.Wait())
}
