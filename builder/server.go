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

func (*server) Build(_ context.Context, req *pb.Project) (*pb.Project, error) {
	//Terraformer
	project := messageTerraformer(req) //sending request(project) to Terraformer
	//Wireguardian
	_, err := messageWireguardian(project) //sending request(project) to Wireguardian
	if err != nil {
		log.Fatalln("Building Wireguard VPN was unsuccessful")
	}
	log.Println("OK")
	//KubeEleven

	return project, nil //return response(project) to the client(Reconcilliator)
}

func main() {
	fmt.Println("Builder server is running")

	lis, err := net.Listen("tcp", ports.BuilderPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.RegisterBuildServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
