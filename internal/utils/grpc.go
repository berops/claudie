package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Error().Msgf("Error while closing the client connection %s : %w", connection.Target(), err)
	}
}

func GrpcDialWithInsecure(serviceName string, serviceURL string) (*grpc.ClientConn, error) {
	cc, err := grpc.Dial(serviceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", serviceName, err)
	}

	return cc, nil
}

// GrpcDialWithInsecureAndBackoff is a blocking function that creates an insecure
// gRPC connection to serviceURL with a retry algorithm that on connection failure
// retries to reconnect. The timer deadline, after which the algorithm should stop
// trying to reconnect, is taken from the ctx parameter, it's expected that the passed
// in ctx parameter was created with context.WithTimeout. If no deadline is supplied the
// reconnecting algorithm runs in a loop forever until the service is available again.
// After successfully connected, any RPC calls made from this connection also have a retry
// policy of 1 minute after which an error is returned that it couldn't connect to the service.
func GrpcDialWithInsecureAndBackoff(ctx context.Context, serviceName, serviceURL string) (*grpc.ClientConn, error) {
	// retry policy for RPC calls.
	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(4 * time.Second)),
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.Unavailable),
	}

	conn, err := grpc.DialContext(
		ctx,
		serviceURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
	)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("failed to connect to service %s within a graceful period of time: %w", serviceName, err)
		}
		return nil, fmt.Errorf("could not connect to %s: %w", serviceName, err)
	}

	return conn, nil
}
