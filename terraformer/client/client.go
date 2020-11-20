package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"

	"google.golang.org/grpc"
)

func main() {
	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure()) //connects to a grpc server
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close() //close the connection after response is received

	c := pb.NewBuildInfrastructureServiceClient(cc)

	project := &pb.Project{
		Metadata: &pb.Metadata{
			Name: "Test",
			Id:   "12345",
		},
		PrivateKey: "/Users/samuelstolicny/go/src/github.com/Berops/platform/terraformer/keys/keykey",
		PublicKey:  "/Users/samuelstolicny/go/src/github.com/Berops/platform/terraformer/keys/keykey.pub",
		Cluster: &pb.Cluster{
			Network: &pb.Network{
				Ip:   "192.168.2.0",
				Mask: "24",
			},
			Nodes: []*pb.Node{
				{
					PrivateIp:      "192.168.2.1",
					Provider:       "hetzner",
					IsControlPlane: true,
					ServerType:     "cpx11",
				},
				{
					PrivateIp:      "192.168.2.2",
					Provider:       "hetzner",
					IsControlPlane: true,
					ServerType:     "cpx11",
				},
				{
					PrivateIp:      "192.168.2.3",
					Provider:       "hetzner",
					IsControlPlane: false,
					ServerType:     "cpx11",
				},
				{
					PrivateIp:      "192.168.2.4",
					Provider:       "hetzner",
					IsControlPlane: false,
					ServerType:     "cpx11",
				},
			},
			KubernetesVersion: "v1.19.0",
		},
	}

	buildInfrastructure(c, project)
}

func buildInfrastructure(c pb.BuildInfrastructureServiceClient, project *pb.Project) {
	fmt.Println("Starting to do a Unary RPC")
	req := project

	res, err := c.BuildInfrastructure(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		log.Fatalln("error while calling BuildVPN RPC", err)
	}
	log.Println("Infrastructure was successfully built", res)
}
