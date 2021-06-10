package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildInfrastructure(ctx context.Context, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	fmt.Println("BuildInfrastructure function was invoked with", req)
	config := req.GetConfig()
	currentState, err := buildInfrastructure(config.GetDesiredState())
	if err != nil {
		log.Fatalln("Template generator failed:", err)
	}
	config.CurrentState, config.DesiredState = currentState, currentState // Update currentState and desiredState
	log.Println("Infrastructure was successfully generated")
	return &pb.BuildInfrastructureResponse{Config: config}, nil
}

func main() {
	// If code crash, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	lis, err := net.Listen("tcp", ports.TerraformerPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("Terraformer service is listening on", ports.TerraformerPort)

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
