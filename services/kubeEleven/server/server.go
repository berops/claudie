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

func (*server) BuildCluster(_ context.Context, project *pb.Project) (*pb.Project, error) {
	fmt.Println("BuildCluster function was invoked with", project)

	generateKubeConfiguration("./templates/kubeone.tpl", "./kubeone/kubeone.yaml", project)
	runKubeOne()
	project.Cluster.Kubeconfig = getKubeconfig()

	//fmt.Println("Kubeconfig:", string(req.GetCluster().GetKubeconfig()))
	return project, nil
}

func main() {
	fmt.Println("KubeEleven server is listening on", ports.KubeElevenPort)

	lis, err := net.Listen("tcp", ports.KubeElevenPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.RegisterKubeElevenServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
