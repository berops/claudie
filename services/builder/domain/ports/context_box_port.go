package ports

import "github.com/berops/claudie/proto/pb"

type ContextBoxPort interface {
	GetConfigBuilder(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error)
	SaveConfigBuilder(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error
	SaveWorkflowState(configName, clusterName string, wf *pb.Workflow, contextBoxGrpcClient pb.ContextBoxServiceClient) error
	DeleteConfig(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error

	PerformHealthCheck() error
	GetClient() pb.ContextBoxServiceClient
}
