package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct{}

func (*server) BuildInfrastructure(ctx context.Context, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	fmt.Println("BuildInfrastructure function was invoked with config", req.GetConfig().GetName())
	config := req.GetConfig()
	err := buildInfrastructure(config.GetDesiredState())
	if err != nil {
		return nil, fmt.Errorf("template generator failed: %v", err)
	}
	log.Println("Infrastructure was successfully generated")
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
	// If code crash, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Set the context-box port
	terraformerPort := os.Getenv("TERRAFORMER_PORT")
	if terraformerPort == "" {
		terraformerPort = "50052" // Default value
	}

	// Start Terraformer Service
	lis, err := net.Listen("tcp", "0.0.0.0:"+terraformerPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("Terrafomer service is listening on:", "0.0.0.0:"+terraformerPort)

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

	log.Println("Stopping Terraformer: ", g.Wait())
}
