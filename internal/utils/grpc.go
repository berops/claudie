package utils

import (
	"fmt"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"math"
	"time"
)

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Err(err).Msgf("Error while closing the client connection %s", connection.Target())
	}
}

func NewGRPCServer() *grpc.Server {
	return grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             45 * time.Second, // If a client pings more than once every 45 seconds, terminate the connection
			PermitWithoutStream: true,             // Allow pings even when there are no active streams
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     math.MaxInt64,   // If a client is idle for INFINITE seconds, send a GOAWAY.
			MaxConnectionAge:      math.MaxInt64,   // If any connection is alive for more than INIFINITE seconds, send a GOAWAY.
			MaxConnectionAgeGrace: math.MaxInt64,   // Allow INIFNITE seconds for pending RPCs to complete before forcibly closing connections.
			Time:                  2 * time.Hour,   // Ping the client if it is idle for 2 Hours to ensure the connection is still active.
			Timeout:               5 * time.Minute, // Wait 5 minutes for the ping ack before assuming the connection is dead.
		}),
	)
}

// GrpcDialWithRetryAndBackoff creates an insecure gRPC connection to serviceURL
// After successfully connected, any RPC calls made from this connection also have a retry
// policy of ~10 minutes after which an error is returned that it couldn't connect to the service.
func GrpcDialWithRetryAndBackoff(serviceName, serviceURL string) (*grpc.ClientConn, error) {
	kacp := keepalive.ClientParameters{
		Time:                45 * time.Second, // ping the server every ~45 seconds.
		Timeout:             5 * time.Minute,  // wait up to 5 minutes for a packet to be acknowledged.
		PermitWithoutStream: true,             // send pings if there are no active streams (RPCs).
	}

	// retry policy for RPC calls.
	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffLinearWithJitter(45*time.Second, 0.1)),
		grpc_retry.WithMax(6),
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
