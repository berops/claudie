package utils

import (
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Fatalln("Error while closing the client connection:", err)
	}
}

func GrpcDialWithInsecure(serviceName string, serviceURL string) (*grpc.ClientConn, error) {
	cc, err := grpc.Dial(serviceURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Could not connect to %s: %v", serviceName, err)
	} else {
		return cc, err
	}
}

func SetConfigErrorMessage(config *pb.Config, err error) *pb.Config {
	if config.Status == nil {
		config.Status = new(pb.Config_Status)
	}
	config.Status.IsFail = true
	config.Status.ErrorMessage = err.Error()
	return config
}
