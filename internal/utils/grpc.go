package utils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
)

var ErrConnectionNotReady = errors.New("unhealthy gRPC connection")

// CloseClientConnection is a wrapper around grpc.ClientConn Close function
func CloseClientConnection(connection *grpc.ClientConn) {
	if err := connection.Close(); err != nil {
		log.Err(err).Msgf("Error while closing the client connection %s", connection.Target())
	}
}

func PeerInfoInterceptor(logger *zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		p, ok := peer.FromContext(ctx)
		if ok {
			peerAddr := p.Addr.String()
			logger.Debug().Msgf("incoming request: %v from peer connected with addr: %s\n", info.FullMethod, peerAddr)
		}
		if !ok {
			logger.Debug().Msg("Peer information cannot be extracted")
		}

		return handler(ctx, req)
	}
}

func NewGRPCServer(opts ...grpc.ServerOption) *grpc.Server {
	opts = append(opts,
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // If a client doesn't wait at least 5 seconds before a ping terminate.
			PermitWithoutStream: true,            // Allow pings even when there are no active streams
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     math.MaxInt64,    // If a client is idle for INFINITE seconds, send a GOAWAY.
			MaxConnectionAge:      math.MaxInt64,    // If any connection is alive for more than INFINITE seconds, send a GOAWAY.
			MaxConnectionAgeGrace: math.MaxInt64,    // Allow INFINITE seconds for pending RPCs to complete before forcibly closing connections.
			Time:                  2 * time.Hour,    // Ping the client if it is idle for 2 Hours to ensure the connection is still active.
			Timeout:               20 * time.Second, // Wait 20 seconds for the ping ack before assuming the connection is dead.
		}),
	)

	return grpc.NewServer(opts...)
}

// GrpcDialWithRetryAndBackoff creates an insecure gRPC connection to serviceURL
// After successfully connected, any RPC calls made from this connection also have a retry
// policy of ~10 minutes after which an error is returned that it couldn't connect to the service.
func GrpcDialWithRetryAndBackoff(serviceName, serviceURL string) (*grpc.ClientConn, error) {
	kacp := keepalive.ClientParameters{
		Time:                20 * time.Second, // ping the server every ~20 seconds.
		Timeout:             20 * time.Second, // wait up to 20 seconds for a packet to be acknowledged.
		PermitWithoutStream: true,             // send pings if there are no active streams (RPCs).
	}

	// retry policy for RPC calls.
	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffLinearWithJitter(45*time.Second, 0.1)),
		grpc_retry.WithMax(3),
		grpc_retry.WithCodes(codes.Unavailable),
	}

	conn, err := grpc.NewClient(
		serviceURL,
		grpc.WithKeepaliveParams(kacp),
		grpc.WithIdleTimeout(0), // Disable idle timeout will try to keep the connection active at all times.
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
	)

	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", serviceName, err)
	}

	return conn, nil
}

func IsConnectionReady(c *grpc.ClientConn) error {
	if c.GetState() != connectivity.Ready {
		return ErrConnectionNotReady
	}
	return nil
}
