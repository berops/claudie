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

	c := pb.NewBuildVPNServiceClient(cc)

	project := &pb.Project{
		Metadata: &pb.Metadata{
			Name: "Test",
			Id:   "12345",
		},
		Cluster: &pb.Cluster{
			Network: &pb.Network{
				Ip:   "192.168.2.0",
				Mask: "24",
			},
			Nodes: []*pb.Node{
				{
					PublicIp:       "168.119.170.52",
					PrivateIp:      "192.168.2.1",
					IsControlPlane: true,
				},
				{
					PublicIp:       "168.119.173.167",
					PrivateIp:      "192.168.2.2",
					IsControlPlane: true,
				},
				{
					PublicIp:       "168.119.169.217",
					PrivateIp:      "192.168.2.3",
					IsControlPlane: false,
				},
				{
					PublicIp:       "168.119.173.20",
					PrivateIp:      "192.168.2.4",
					IsControlPlane: false,
				},
			},
			KubernetesVersion: "v1.19.0",
		},
	}

	buildVPN(c, project)
}

func buildVPN(c pb.BuildVPNServiceClient, project *pb.Project) {
	fmt.Println("Starting to do a Unary RPC")
	req := project

	res, err := c.BuildVPN(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		log.Fatalln("error while calling BuildVPN RPC", err)
	}
	log.Println("BuildVPN success status:", res.GetSuccess())
}
