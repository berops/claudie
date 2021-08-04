package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildInfrastructure(ctx context.Context, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	fmt.Println("BuildInfrastructure function was invoked with config", req.GetConfig().GetName())
	config := req.GetConfig()
	err := buildInfrastructure(config)
	if err != nil {
		log.Fatalln("Template generator failed:", err)
	}
	log.Println("Infrastructure was successfully generated")
	return &pb.BuildInfrastructureResponse{Config: config}, nil
}

func (*server) DestroyInfrastructure(ctx context.Context, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	fmt.Println("DestroyInfrastructure function was invoked with config:", req.GetConfig().GetName())
	config := req.GetConfig()
	err := destroyInfrastructure(config.GetCurrentState())
	if err != nil {
		log.Fatalln("Error while destroying the infrastructure", err)
	}
	res := &pb.DestroyInfrastructureResponse{Config: config}
	return res, nil
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

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Block until a signal is received
	<-ch
	fmt.Println("Stopping the server")
	s.Stop()
	fmt.Println("Closing the listener")
	lis.Close()
	fmt.Println("End of Program")

}
