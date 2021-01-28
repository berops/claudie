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

	c := pb.NewBuildClusterServiceClient(cc)

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
			Count:      1,
			ServerType: "f1-micro",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		ComputeNodeSpecs: &pb.ComputeNodeSpecs{
			Count:      2,
			ServerType: "f1-micro",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		IsInUse: true,
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
					PublicIp:  "95.216.162.187",
					PrivateIp: "192.168.2.1",
					IsWorker:  false,
				},
				{
					PublicIp:  "95.216.160.148",
					PrivateIp: "192.168.2.2",
					IsWorker:  true,
				},
				{
					PublicIp:  "95.216.161.182",
					PrivateIp: "192.168.2.3",
					IsWorker:  true,
				},
				{
					PublicIp:  "34.76.125.174",
					PrivateIp: "192.168.2.4",
					IsWorker:  false,
				},
				{
					PublicIp:  "35.195.47.33",
					PrivateIp: "192.168.2.5",
					IsWorker:  true,
				},
				{
					PublicIp:  "34.77.235.6",
					PrivateIp: "192.168.2.6",
					IsWorker:  true,
				},
			},
			KubernetesVersion: "1.19.0",
			Providers:         providers,
			PrivateKey:        "../../keys/testkey",
			PublicKey:         "../../keys/testkey.pub",
		},
	}

	createCluster(c, project)
}

func createCluster(c pb.BuildClusterServiceClient, project *pb.Project) {
	fmt.Println("Sending project to KubeEleven...")
	req := project

	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Fatalln("Error received from KubeEleven:", err)
	}
	log.Println("Cluster was created", res)

}
