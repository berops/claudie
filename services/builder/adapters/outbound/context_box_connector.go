package outbound

import (
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
	"google.golang.org/grpc"
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

// GetConfigBuilder requests a new config for builder from context-box.
func (c *ContextBoxConnector) GetConfigBuilder(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	return cbox.GetConfigBuilder(contextBoxGrpcClient)
}

// SaveConfigBuilder saves a config from builder to Claudie database via context-box.
func (c *ContextBoxConnector) SaveConfigBuilder(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error {
	return cbox.SaveConfigBuilder(contextBoxGrpcClient, &pb.SaveConfigRequest{Config: config})
}

// SaveWorkflowState saves workflow state for a particular cluster.
func (c *ContextBoxConnector) SaveWorkflowState(configName, clusterName string, wf *pb.Workflow, contextBoxGrpcClient pb.ContextBoxServiceClient) error {
	return cbox.SaveWorkflowState(contextBoxGrpcClient, &pb.SaveWorkflowStateRequest{ConfigName: configName, ClusterName: clusterName, Workflow: wf})
}

// DeleteConfig removes config from Claudie database via context-box.
func (c *ContextBoxConnector) DeleteConfig(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error {
	return cbox.DeleteConfigFromDB(contextBoxGrpcClient, &pb.DeleteConfigRequest{Id: config.Id, Type: pb.IdType_HASH})
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

// GetClient returns a context-box gRPC client.
func (c *ContextBoxConnector) GetClient() pb.ContextBoxServiceClient {
	return pb.NewContextBoxServiceClient(c.Connection)
}
