package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/urls"

	"google.golang.org/grpc"
)

func messageWireguardian(project *pb.Project) (*pb.Status, error) {
	cc, err := grpc.Dial(urls.WireguardianURL, grpc.WithInsecure()) //connects to a grpc server
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close() //close the connection after response is received

	c := pb.NewWireguardianServiceClient(cc)

	return buildVPN(c, project) //returns success status to Builder module
}

func buildVPN(c pb.WireguardianServiceClient, project *pb.Project) (*pb.Status, error) {
	fmt.Println("Sending a project message to Wireguardian")
	req := project

	res, err := c.BuildVPN(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		log.Fatalln("error while BuildVPN RPC, probably due to unsuccessful ansible in wireguardian", err)
	}
	log.Println("BuildVPN success status:", res.GetSuccess())
	return res, nil
}
