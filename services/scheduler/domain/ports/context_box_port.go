package ports

import "github.com/berops/claudie/proto/pb"

type ContextBoxPort interface {
	GetConfigScheduler(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error)
	SaveConfigScheduler(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error
}
