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
	//Check if project has a kubeconfig
	if req.GetCluster().GetKubeconfig() != nil { // Kubeconfig exists
		fmt.Println("Cluster already has a kubeconfig file")
		//Check if there are any different nodes and delete them
		err := deleteNodes(req)
		if err != nil {
			log.Fatalln("Error while deleting nodes from cluster", err)
		}
	}

	//Call TWK modules
	project, err := flow(req)
	if err != nil {
		log.Fatalln("Error in flow: ", err)
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

// flow permorms the sequence of gRPC calls to Terraformer, Wireguardian, KubeEleven modules (TWK)
func flow(project *pb.Project) (*pb.Project, error) {
	//Terraformer
	project, err := messageTerraformer(project) //sending project message to Terraformer
	if err != nil {
		log.Fatalln("Error while Building Infrastructure", err)
	}
	//Wireguardian
	_, err = messageWireguardian(project) //sending project message to Wireguardian
	if err != nil {
		log.Fatalln("Error while creating Wireguard VPN", err)
	}
	//KubeEleven
	project, err = messageKubeEleven(project) //sending project message to KubeEleven
	if err != nil {
		log.Fatalln("Error while creating the cluster with KubeOne", err)
	}

	return project, nil
}
