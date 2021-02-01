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

func (*server) BuildCluster(_ context.Context, req *pb.Project) (*pb.Project, error) {
	fmt.Println("BuildCluster function was invoked with", req)
	generateKubeConfiguration("./templates/kubeone.tpl", "./kubeone/kubeone.yaml", req)
	runKubeOne()
	req.Cluster.Kubeconfig = getKubeconfig()
	fmt.Println("Kubeconfig:", string(req.GetCluster().GetKubeconfig()))
	return req, nil
}

func main() {
	fmt.Println("KubeEleven server is running")

	lis, err := net.Listen("tcp", "localhost:50054")
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.RegisterBuildClusterServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
