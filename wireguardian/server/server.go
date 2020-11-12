package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Berops/wireguardian/inventory"
	"github.com/Berops/wireguardian/wireguardianpb"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildVPN(_ context.Context, req *wireguardianpb.Project) (*wireguardianpb.Status, error) {
	fmt.Println("BuildVPN function was invoked with", req)
	var nodes []*wireguardianpb.Node //creates empty slice of nodes
	//nodes = append(nodes, req.GetControlPlane()...)
	nodes = append(nodes, req.GetCluster().GetControlPlane()...)
	nodes = append(nodes, req.GetCluster().GetComputePlane()...)
	inventory.Generate(nodes)
	err := runAnsible()
	if err != nil {
		return &wireguardianpb.Status{Success: false}, nil
	}
	return &wireguardianpb.Status{Success: true}, nil
}

func main() {
	fmt.Println("wireguardian_api server is running")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	// creating a new server
	s := grpc.NewServer()
	wireguardianpb.RegisterBuildVPNServiceServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
