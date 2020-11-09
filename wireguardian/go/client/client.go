package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/wireguardian/wireguardianpb"
	"google.golang.org/grpc"
)

func main() {
	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure()) //connects to a grpc server
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close() //close the connection after response is received

	c := wireguardianpb.NewBuildVPNServiceClient(cc)

	buildVPN(c)
}

func buildVPN(c wireguardianpb.BuildVPNServiceClient) {
	fmt.Println("Starting to do a Unary RPC")
	req := &wireguardianpb.Cluster{
		ControlPlane: []*wireguardianpb.Node{
			{
				PublicIp:  "159.69.9.154",
				PrivateIp: "192.168.2.1",
			},
			{
				PublicIp:  "168.119.119.101",
				PrivateIp: "192.168.2.2",
			},
		},
		ComputePlane: []*wireguardianpb.Node{
			{
				PublicIp:  "159.69.29.78",
				PrivateIp: "192.168.2.3",
			},
			{
				PublicIp:  "168.119.115.229",
				PrivateIp: "192.168.2.4",
			},
		},
		KubernetesVersion: "v1.19.0",
	}

	res, err := c.BuildVPN(context.Background(), req)
	if err != nil {
		log.Fatalln("error while calling BuildVPN RPC", err)
	}
	log.Println("Response from BuildVPN", res.GetResponse())
}
