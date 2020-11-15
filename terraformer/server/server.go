package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildInfrastructure(_ context.Context, req *pb.Project) (*pb.Project, error) {
	fmt.Println("BuildInfrastructure function was invoked with", req)

}

func main() {
	fmt.Println("test")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.BuildInfrastructureServiceServer(s, &server{})
}
