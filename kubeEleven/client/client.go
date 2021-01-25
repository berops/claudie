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

	c := pb.NewCreateClusterServiceClient(cc)

	providers := make(map[string]*pb.Provider)
	providers["hetzner"] = &pb.Provider{
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
			ServerType: "f1-micro",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		ComputeNodeSpecs: &pb.ComputeNodeSpecs{
			Count:      0,
			ServerType: "f1-micro",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		IsInUse: false,
	}

	project := &pb.Project{
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
					PublicIp:  "135.181.100.212",
					PrivateIp: "192.168.2.1",
				},
				{
					PublicIp:  "135.181.40.234",
					PrivateIp: "192.168.2.2",
				},
				{
					PublicIp:  "135.181.37.13",
					PrivateIp: "192.168.2.3",
				},
			},
			KubernetesVersion: "1.19.0",
			Providers:         providers,
			PrivateKey:        "/Users/samuelstolicny/go/src/github.com/Berops/platform/keys/keykey",
			PublicKey:         "/Users/samuelstolicny/go/src/github.com/Berops/platform/keys/keykey.pub",
			ControlCount:      1,
			WorkerCount:       1,
		},
	}

	createCluster(c, project)
}

func createCluster(c pb.CreateClusterServiceClient, project *pb.Project) {
	fmt.Println("Sending project to KubeEleven...")
	req := project

	res, err := c.CreateCluster(context.Background(), req)
	if err != nil {
		log.Fatalln("Error received from KubeEleven:", err)
	}
	log.Println("Cluster was created", res)

}
