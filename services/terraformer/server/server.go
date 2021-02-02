package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildInfrastructure(_ context.Context, req *pb.Project) (*pb.Project, error) {
	fmt.Println("BuildInfrastructure function was invoked with", req)

	err := buildTerraform(req)
	if err != nil {
		log.Fatalln("Template generator failed:", err)
	}

	log.Println("Infrastructure was successfully generated")
	return req, nil
}

func main() {
	fmt.Println("Terraformer server is listening on", ports.TerraformerPort)

	lis, err := net.Listen("tcp", ports.TerraformerPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.RegisterBuildInfrastructureServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
