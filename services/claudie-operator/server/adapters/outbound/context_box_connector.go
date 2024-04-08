package outboundAdapters

import (
	"google.golang.org/grpc"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
)

// Communicates with the gRPC server of context-box microservice
type ContextBoxConnector struct {
	connectionUri string

	// grpcConnection is the underlying gRPC connection to context-box microservice.
	grpcConnection *grpc.ClientConn

	// grpcClient is a gRPC client connection to context-box microservice.
	grpcClient pb.ContextBoxServiceClient
}

// NewContextBoxConnector creates and returns an instance of the ContextBoxConnector struct
func NewContextBoxConnector(connectionUri string) *ContextBoxConnector {
	return &ContextBoxConnector{
		connectionUri: connectionUri,
	}
}

// Connect creates a gRPC connection to the context-box microservice.
// If the connection is established, then performs a healthcheck.
func (c *ContextBoxConnector) Connect() error {
	grpcConnection, err := utils.GrpcDialWithRetryAndBackoff("context-box", c.connectionUri)
	if err != nil {
		return err
	}

	c.grpcConnection = grpcConnection
	c.grpcClient = pb.NewContextBoxServiceClient(grpcConnection)

	return nil
}

// PerformHealthCheck checks health of the underlying gRPC connection to context-box microservice
func (c *ContextBoxConnector) PerformHealthCheck() error {
	return utils.IsConnectionReady(c.grpcConnection)
}

// GetAllConfigs fetches all configs present in context-box DB
func (c *ContextBoxConnector) GetAllConfigs() ([]*pb.Config, error) {
	response, err := cbox.GetAllConfigs(c.grpcClient)
	if err != nil {
		return nil, err
	}

	return response.GetConfigs(), nil
}

// SaveConfig sends request to the context-box microservice, to save a config in context-box DB.
func (c *ContextBoxConnector) SaveConfig(config *pb.Config) error {
	_, err := cbox.SaveConfigOperator(c.grpcClient, &pb.SaveConfigRequest{Config: config})
	return err
}

// DeleteConfig sends request to the context-box microservice, to delete a config with the given id, from context-box DB.
func (c *ContextBoxConnector) DeleteConfig(configName string) error {
	return cbox.DeleteConfig(c.grpcClient,
		&pb.DeleteConfigRequest{
			Id:   configName,
			Type: pb.IdType_NAME,
		},
	)
}

// Disconnect closes the gRPC connection to context-box microservice
func (c *ContextBoxConnector) Disconnect() {
	utils.CloseClientConnection(c.grpcConnection)
}
