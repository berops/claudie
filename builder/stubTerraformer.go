package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

func messageTerraformer(project *pb.Project) {
	cc, err := grpc.Dial("localhost:50052", grpc.WithInsecure()) //connects to a grpc server
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close() //close the connection after response is received

	c := pb.NewBuildInfrastructureServiceClient(cc)

	buildInfrastructure(c, project)
}

func buildInfrastructure(c pb.BuildInfrastructureServiceClient, project *pb.Project) {
	fmt.Println("Starting to do a Unary RPC")
	req := project

	res, err := c.BuildInfrastructure(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		log.Fatalln("error while calling BuildInfrastructure on Terraformer", err)
	}
	log.Println("Infrastructure was successfully built", res)

	for _, ip := range res.GetCluster().GetNodes() {
		fmt.Println(ip)
	}
}
