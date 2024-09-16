package ports

import (
	"github.com/berops/claudie/proto/pb"
	builder "github.com/berops/claudie/services/builder/internal"
)

type KubeElevenPort interface {
	BuildCluster(builderCtx *builder.Context, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.BuildClusterResponse, error)
	DestroyCluster(builderCtx *builder.Context, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.DestroyClusterResponse, error)

	PerformHealthCheck() error
	GetClient() pb.KubeElevenServiceClient
}
