package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/serializer"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) Build(_ context.Context, req *pb.Project) (*pb.Project, error) {
	//Terraformer
	// project, err := messageTerraformer(req) //sending request(project) to Terraformer
	// if err != nil {
	// 	log.Fatalln("Error while Building Infrastructure", err)
	// }
	//time.Sleep(10 * time.Second) //sleep because newly created servers are not ready yet
	//Wireguardian
	// _, err = messageWireguardian(project) //sending request(project) to Wireguardian
	// if err != nil {
	// 	log.Fatalln("Error while creating Wireguard VPN", err)
	// }
	//KubeEleven
	project, err := messageKubeEleven(req) //sending request(project) to KubeEleven
	if err != nil {
		log.Fatalln("Error while creating the cluster with KubeOne", err)
	}

	// Saving file TEMPORARY
	err = serializer.WriteProtobufToBinaryFile(project, "../../tmp/project.bin")
	if err != nil {
		log.Fatalln("Error while saving the project a binary file", err)
	}
	log.Println("Project has been saved to a binary file")
	err = serializer.WriteProtobufToJSONFile(project, "../../tmp/project.json")
	if err != nil {
		log.Fatalln("Error while saving the project a json file", err)
	}
	log.Println("Project has been saved to a json file")

	return project, nil //return response(project) to the client(Reconcilliator)
}

func main() {
	fmt.Println("Builder server is running on ", ports.BuilderPort)

	lis, err := net.Listen("tcp", ports.BuilderPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}

	s := grpc.NewServer()
	pb.RegisterBuilderServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
