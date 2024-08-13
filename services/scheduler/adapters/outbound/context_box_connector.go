package outboundAdapters

import (
	"google.golang.org/grpc"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	cbox "github.com/berops/claudie/services/context-box/client"
)

type ContextBoxConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the context-box microservice
func (c *ContextBoxConnector) Connect() error {
	connection, err := utils.GrpcDialWithRetryAndBackoff("context-box", envs.ContextBoxURL)
	if err != nil {
		return err
	}
	c.Connection = connection

	return nil
}

// The context-box microservice has a scheduler queue(FIFO) containing ConfigInfos
// of those configs whose desired state needs to be built.
// GetConfigScheduler gets config from context-box DB corresponding to the configInfo
// present in the front of the scheduler queue.
func (c ContextBoxConnector) GetConfigScheduler(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	return cbox.GetConfigScheduler(contextBoxGrpcClient)
}

// SaveConfigScheduler saves a config into context-box DB
func (c ContextBoxConnector) SaveConfigScheduler(config *spec.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error {
	return cbox.SaveConfigScheduler(contextBoxGrpcClient, &pb.SaveConfigRequest{Config: config})
}

// Disconnect closes the underlying gRPC connection to context-box microservice
func (c *ContextBoxConnector) Disconnect() {
	utils.CloseClientConnection(c.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to context-box microservice.
func (c *ContextBoxConnector) PerformHealthCheck() error {
	if err := utils.IsConnectionReady(c.Connection); err == nil {
		return nil
	} else {
		c.Connection.Connect()
		return err
	}
}
