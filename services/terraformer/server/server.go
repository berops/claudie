package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/urls"
	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildInfrastructure(_ context.Context, req *pb.Project) (*pb.Project, error) {
	fmt.Println("BuildInfrastructure function was invoked with", req)

	err := buildTerraform(req)
	if err != nil {
		log.Fatalln("Template generator failed:", err)
	}

	log.Println("Infrastructure was successfully generated")
	return req, nil
}

func main() {
	// If we crath the go gode, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	lis, err := net.Listen("tcp", urls.TerraformerURL)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("Terraformer service is listening on", urls.TerraformerURL)

	s := grpc.NewServer()
	pb.RegisterTerraformerServiceServer(s, &server{})

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
