package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"github.com/Berops/platform/proto/pb"
)

type server struct {}

func (*server) Build(_ context.Context, req *pb.Project) (*pb.Project, error) {

	messageTerraformer(req)
	fmt.Println("success")
	return req, nil
}

func main() {
	fmt.Println("Builder server is running")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.RegisterBuildServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}