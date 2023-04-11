package outboundAdapters

import (
	"errors"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/berops/claudie/proto/pb"
)

// communicates with the gRPC server of context-box microservice
type ContextBoxConnector struct {
	connectionUri string

	// grpcConnection is the underlying gRPC connection to context-box microservice.
	grpcConnection *grpc.ClientConn

	// GrpcClient is a gRPC client connection to context-box microservice.
	GrpcClient pb.ContextBoxServiceClient
}

func NewContextBoxConnector(connectionUri string) *ContextBoxConnector {
	return &ContextBoxConnector{
		connectionUri: connectionUri,
	}
}

func (c *ContextBoxConnector) Connect() error {

	// since the k8sSidecarNotificationsReceiver will be responding to incoming notifications we can't
	// use a blocking gRPC dial to the context-box service. Thus we default to a non-blocking
	// connection with a retry policy of ~4 seconds instead.
	interceptorOptions := []grpc_retry.CallOption{

		grpc_retry.WithBackoff(grpc_retry.BackoffExponentialWithJitter(4*time.Second, 0.2)),
		grpc_retry.WithMax(7),
		grpc_retry.WithCodes(codes.Unavailable),
	}

	grpcConnection, err := grpc.Dial(
		c.connectionUri,
		grpc.WithTransportCredentials(insecure.NewCredentials()),

		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(interceptorOptions...)),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(interceptorOptions...)),
	)
	if err != nil {
		return err
	}

	c.grpcConnection = grpcConnection
	c.GrpcClient = pb.NewContextBoxServiceClient(grpcConnection)

	return c.PerformHealthCheck()

}

func (c *ContextBoxConnector) PerformHealthCheck() error {
	if c.grpcConnection.GetState() == connectivity.Shutdown {
		return errors.New("Unhealthy gRPC connection to context-box microservice")
	}

	return nil
}

func (c *ContextBoxConnector) Disconnect() error {
	return c.grpcConnection.Close()
}
