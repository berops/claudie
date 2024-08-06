package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
)

type ContextBoxPort interface {
	GetConfigBuilder(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error)
	SaveConfigBuilder(config *spec.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error
	SaveWorkflowState(configName, clusterName string, wf *spec.Workflow, contextBoxGrpcClient pb.ContextBoxServiceClient) error
	DeleteConfig(config *spec.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error

	PerformHealthCheck() error
	GetClient() pb.ContextBoxServiceClient
}
