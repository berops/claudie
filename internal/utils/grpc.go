package utils

import (
	"fmt"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"time"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Err(err).Msgf("Error while closing the client connection %s", connection.Target())
	}
}

func GrpcDialWithInsecure(serviceName string, serviceURL string) (*grpc.ClientConn, error) {
	kacp := keepalive.ClientParameters{
		Time:                60 * time.Second, // ping the server every ~60 seconds.
		Timeout:             20 * time.Minute, // wait up to 20 minutes for a packet to be acknowledged.
		PermitWithoutStream: true,             // send pings if there are no active streams (RPCs).
	}

	cc, err := grpc.Dial(serviceURL, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithKeepaliveParams(kacp))
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", serviceName, err)
	}

	return cc, nil
}

// GrpcDialWithRetryAndBackoff creates an insecure gRPC connection to serviceURL
// After successfully connected, any RPC calls made from this connection also have a retry
// policy of ~10 minutes after which an error is returned that it couldn't connect to the service.
func GrpcDialWithRetryAndBackoff(serviceName, serviceURL string) (*grpc.ClientConn, error) {
	kacp := keepalive.ClientParameters{
		Time:                60 * time.Second, // ping the server every ~60 seconds.
		Timeout:             20 * time.Minute, // wait up to 20 minutes for a packet to be acknowledged.
		PermitWithoutStream: true,             // send pings if there are no active streams (RPCs).
	}

	// retry policy for RPC calls.
	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffLinearWithJitter(45*time.Second, 0.1)),
		grpc_retry.WithMax(15),
		grpc_retry.WithCodes(codes.Unavailable),
	}

	conn, err := grpc.Dial(
		serviceURL,
		grpc.WithKeepaliveParams(kacp),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
	)

	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", serviceName, err)
	}

	return conn, nil
}
