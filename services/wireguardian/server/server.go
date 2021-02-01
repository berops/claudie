package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/wireguardian/inventory"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildVPN(_ context.Context, req *pb.Project) (*pb.Status, error) {
	fmt.Println("BuildVPN function was invoked with", req)

	inventory.Generate(req.GetCluster().GetNodes())
	err := runAnsible(req)
	if err != nil {
		return &pb.Status{Success: false}, nil
	}
	return &pb.Status{Success: true}, nil
}

func main() {
	fmt.Println("wireguardian_api server is running")

	lis, err := net.Listen("tcp", ports.WireguardianPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	// creating a new server
	s := grpc.NewServer()
	pb.RegisterBuildVPNServiceServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
