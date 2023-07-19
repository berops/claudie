package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

type KubeElevenPort interface {
	BuildCluster(builderCtx *utils.BuilderContext, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.BuildClusterResponse, error)

	PerformHealthCheck() error
	GetClient() pb.KubeElevenServiceClient
}
