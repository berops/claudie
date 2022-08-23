package utils

import (
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Fatalln("Error while closing the client connection:", err)
	}
}

func GrpcDialWithInsecure(serviceName string, serviceURL string) (*grpc.ClientConn, error) {
	cc, err := grpc.Dial(serviceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("Could not connect to %s: %v", serviceName, err)
	} else {
		return cc, nil
	}
}
