package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
)

type ContextBoxPort interface {
	GetConfigScheduler(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error)
	SaveConfigScheduler(config *spec.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error

	PerformHealthCheck() error
}
