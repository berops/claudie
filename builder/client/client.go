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

	c := pb.NewBuildServiceClient(cc)

	providers := make(map[string]*pb.Provider) //create a new map of providers
	providers["hetzner"] = &pb.Provider{       //add new provider to the map
		Name: "hetzner",
		ControlNodeSpecs: &pb.ControlNodeSpecs{
			Count:      1,
			ServerType: "cpx11",
			Image:      "ubuntu-20.04",
		},
		ComputeNodeSpecs: &pb.ComputeNodeSpecs{
			Count:      2,
			ServerType: "cpx11",
			Image:      "ubuntu-20.04",
		},
		IsInUse: true,
	}
	providers["gcp"] = &pb.Provider{
		Name: "gcp",
		ControlNodeSpecs: &pb.ControlNodeSpecs{
			Count:      0,
			ServerType: "e2-small",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		ComputeNodeSpecs: &pb.ComputeNodeSpecs{
			Count:      0,
			ServerType: "f1-micro",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		IsInUse: false,
	}

	project := &pb.Project{ //create a new project
		Metadata: &pb.Metadata{
			Name: "ProjectX",
			Id:   "12345",
		},
		Cluster: &pb.Cluster{
			Network: &pb.Network{
				Ip:   "192.168.2.0",
				Mask: "24",
			},
			Nodes: []*pb.Node{
				{
					PrivateIp: "192.168.2.1",
				},
				{
					PrivateIp: "192.168.2.2",
				},
				{
					PrivateIp: "192.168.2.3",
				},
			},
			KubernetesVersion: "v1.19.0",
			Providers:         providers,
			PrivateKey:        "../keys/testkey",
			PublicKey:         "../keys/testkey.pub",
		},
	}

	build(c, project)
}

func build(c pb.BuildServiceClient, project *pb.Project) {
	fmt.Println("Starting to do a Unary RPC")
	req := project

	res, err := c.Build(context.Background(), req)
	if err != nil {
		log.Fatalln("error while sending message to Builder", err)
	}
	log.Println("Received message from Builder:", res)
}

