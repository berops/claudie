package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

type KuberPort interface {
	SetUpStorage(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) (*pb.SetUpStorageResponse, error)
	StoreLBScrapeConfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	RemoveLBScrapeConfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	StoreClusterMetadata(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	DeleteClusterMetadata(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	StoreKubeconfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	DeleteKubeconfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	SetUpClusterAutoscaler(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	DestroyClusterAutoscaler(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	PatchClusterInfoConfigMap(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	PatchKubeProxyConfigMap(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	PatchNodes(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error
	DeleteNodes(cluster *spec.K8Scluster, masterNodes, workerNodes []string, kuberGrpcClient pb.KuberServiceClient) (*pb.DeleteNodesResponse, error)
	CiliumRolloutRestart(cluster *spec.K8Scluster, kuberGrpcClient pb.KuberServiceClient) error

	PerformHealthCheck() error
	GetClient() pb.KuberServiceClient
}
