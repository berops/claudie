package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

func messageKubeEleven(project *pb.Project) (*pb.Project, error) {
	cc, err := grpc.Dial(ports.KubeElevenPort, grpc.WithInsecure()) //connects to a grpc server
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close() //close the connection after response is received

	c := pb.NewKubeElevenServiceClient(cc)

	return createCluster(c, project)
}

func createCluster(c pb.KubeElevenServiceClient, project *pb.Project) (*pb.Project, error) {
	fmt.Println("Sending project message to KubeEleven")
	req := project

	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Fatalln("Error received from KubeEleven:", err)
	}
	//log.Println("Cluster was created", res)
	return res, nil
}
