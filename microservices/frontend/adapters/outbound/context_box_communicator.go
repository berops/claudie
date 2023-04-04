package outboundAdapters

import (
	"claudie/proto/generated"
	"context"
	"errors"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type ContextBoxConnector struct {

	// grpcConnection is the underlying gRPC connection used
	// by the context-box gRPC client.
	grpcConnection *grpc.ClientConn

	// grpcClient is a gRPC client connection to the
	// context-box service.
	grpcClient generated.ContextBoxClient
}

func(c *ContextBoxConnector) GetConfigList( ) ([]*generated.Config, error) {

	response, err := c.grpcClient.GetConfigList(context.Background( ), &generated.GetConfigListRequest{ })
	if err != nil {
		return []*generated.Config{ }, err}

	return response.GetConfigList( ), nil
}

func(c *ContextBoxConnector) SaveConfig(config *generated.Config) error {

	_, err := c.grpcClient.SaveConfig(context.Background( ),
		&generated.SaveConfigRequest{ Config: config })
	return err
}

func(c *ContextBoxConnector) DeleteConfig(id string) error {

	_, err := c.grpcClient.DeleteConfig(context.Background( ), &generated.DeleteConfigRequest{ Id: id })
	return err
}

func NewContextBoxConnector(connectionUri string) (*ContextBoxConnector, error) {

	// since the k8sSidecarNotificationsReceiver will be responding to incoming requests we can't
	// use a blocking gRPC dial to the context-box service. Thus we default to a non-blocking
	// connection with a RPC retry policy of ~4 minutes instead.
	interceptorOptions := []grpc_retry.CallOption{

		grpc_retry.WithBackoff(grpc_retry.BackoffExponentialWithJitter(4 * time.Second, 0.2)),
		grpc_retry.WithMax(7),
		grpc_retry.WithCodes(codes.Unavailable),
	}

	grpcConnection, err := grpc.Dial(
		connectionUri,
		grpc.WithTransportCredentials(insecure.NewCredentials( )),

		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(interceptorOptions...)),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(interceptorOptions...)),
	)
	if err != nil {
		return nil, err}

	contextBoxConnector := &ContextBoxConnector{
		grpcConnection: grpcConnection,
		grpcClient: generated.NewContextBoxClient(grpcConnection),
	}
	
	return contextBoxConnector, contextBoxConnector.PerformHealthCheck( )
}

func(c *ContextBoxConnector) PerformHealthCheck( ) error {
	if c.grpcConnection.GetState( ) == connectivity.Shutdown {
		return errors.New("Unhealthy connection to context-box")}

	return nil
}

func(c *ContextBoxConnector) Stop( ) error {
	return c.grpcConnection.Close( )}