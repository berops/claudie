package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

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

func main() {
	// If we crath the go gode, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	lis, err := net.Listen("tcp", ports.BuilderPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("Builder service is running on ", ports.BuilderPort)

	s := grpc.NewServer()
	pb.RegisterBuilderServiceServer(s, &server{})

	go func() {
		fmt.Println("Starting Server...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Block until a signal is received
	<-ch
	fmt.Println("Stopping the server")
	s.Stop()
	fmt.Println("Closing the listener")
	lis.Close()
	fmt.Println("End of Program")
}
