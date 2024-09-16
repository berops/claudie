package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
)

type KuberPort interface {
	SetUpStorage(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) (*pb.SetUpStorageResponse, error)
	StoreLBScrapeConfig(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	RemoveLBScrapeConfig(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	StoreClusterMetadata(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	DeleteClusterMetadata(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	StoreKubeconfig(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	DeleteKubeconfig(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	SetUpClusterAutoscaler(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	DestroyClusterAutoscaler(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	PatchClusterInfoConfigMap(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	PatchKubeProxyConfigMap(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	PatchNodes(builderCtx *builder.Context, kuberGrpcClient pb.KuberServiceClient) error
	DeleteNodes(cluster *spec.K8Scluster, masterNodes, workerNodes []string, kuberGrpcClient pb.KuberServiceClient) (*pb.DeleteNodesResponse, error)
	CiliumRolloutRestart(cluster *spec.K8Scluster, kuberGrpcClient pb.KuberServiceClient) error

	PerformHealthCheck() error
	GetClient() pb.KuberServiceClient
}
