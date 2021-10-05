package utils

import (
	"log"

	"google.golang.org/grpc"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Fatalln("Error while closing the client connection:", err)
	}
}
