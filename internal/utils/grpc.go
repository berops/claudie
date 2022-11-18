package utils

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Error().Msgf("Error while closing the client connection %s : %v", connection.Target(), err)
	}
}

func GrpcDialWithInsecure(serviceName string, serviceURL string) (*grpc.ClientConn, error) {
	cc, err := grpc.Dial(serviceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", serviceName, err)
	}

	return cc, nil
}
