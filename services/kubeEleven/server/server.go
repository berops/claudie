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

func (*server) BuildCluster(_ context.Context, project *pb.Project) (*pb.Project, error) {
	fmt.Println("BuildCluster function was invoked with", project)

	generateKubeConfiguration("./templates/kubeone.tpl", "./kubeone/kubeone.yaml", project)
	runKubeOne()
	project.Cluster.Kubeconfig = getKubeconfig()

	//fmt.Println("Kubeconfig:", string(req.GetCluster().GetKubeconfig()))
	return project, nil
}

func main() {
	// If we crath the go gode, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	lis, err := net.Listen("tcp", ports.KubeElevenPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("KubeEleven service is listening on", ports.KubeElevenPort)

	s := grpc.NewServer()
	pb.RegisterKubeElevenServiceServer(s, &server{})

	go func() {
		fmt.Println("Starting Server...")
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
