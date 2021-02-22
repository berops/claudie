package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/wireguardian/inventory"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildVPN(_ context.Context, req *pb.Project) (*pb.Status, error) {
	fmt.Println("BuildVPN function was invoked with", req)

	inventory.Generate(req.GetCluster().GetNodes())
	err := runAnsible(req)
	if err != nil {
		return &pb.Status{Success: false}, nil
	}
	return &pb.Status{Success: true}, nil
}

func main() {
	// If we crath the go gode, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	lis, err := net.Listen("tcp", ports.WireguardianPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("Wireguardian service is listening on", ports.WireguardianPort)

	// creating a new server
	s := grpc.NewServer()
	pb.RegisterWireguardianServiceServer(s, &server{})

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
