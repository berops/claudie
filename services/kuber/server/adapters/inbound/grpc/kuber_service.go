package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/usecases"
)

var _ pb.KuberServiceServer = &KuberGrpcService{}

type KuberGrpcService struct {
	pb.UnimplementedKuberServiceServer

	usecases *usecases.Usecases
}

func (k *KuberGrpcService) PatchKubeProxyConfigMap(ctx context.Context, request *pb.PatchKubeProxyConfigMapRequest) (*pb.PatchKubeProxyConfigMapResponse, error) {
	return k.usecases.PatchKubeProxyConfigMap(ctx, request)
}

func (k *KuberGrpcService) PatchKubeadmConfigMap(ctx context.Context, request *pb.PatchKubeadmConfigMapRequest) (*pb.PatchKubeadmConfigMapResponse, error) {
	return k.usecases.PatchKubeadmConfigMap(ctx, request)
}

func (k *KuberGrpcService) CiliumRolloutRestart(ctx context.Context, request *pb.CiliumRolloutRestartRequest) (*pb.CiliumRolloutRestartResponse, error) {
	return k.usecases.CiliumRolloutRestart(request)
}

func (k *KuberGrpcService) PatchClusterInfoConfigMap(ctx context.Context, request *pb.PatchClusterInfoConfigMapRequest) (*pb.PatchClusterInfoConfigMapResponse, error) {
	return k.usecases.PatchClusterInfoConfigMap(request)
}

func (k *KuberGrpcService) SetUpStorage(ctx context.Context, request *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	return k.usecases.SetUpStorage(ctx, request)
}

func (k *KuberGrpcService) StoreLBScrapeConfig(ctx context.Context, request *pb.StoreLBScrapeConfigRequest) (*pb.StoreLBScrapeConfigResponse, error) {
	return k.usecases.StoreLBScrapeConfig(ctx, request)
}

func (k *KuberGrpcService) RemoveLBScrapeConfig(ctx context.Context, request *pb.RemoveLBScrapeConfigRequest) (*pb.RemoveLBScrapeConfigResponse, error) {
	return k.usecases.RemoveLBScrapeConfig(ctx, request)
}

func (k *KuberGrpcService) StoreClusterMetadata(ctx context.Context, request *pb.StoreClusterMetadataRequest) (*pb.StoreClusterMetadataResponse, error) {
	return k.usecases.StoreClusterMetadata(ctx, request)
}

func (k *KuberGrpcService) DeleteClusterMetadata(ctx context.Context, request *pb.DeleteClusterMetadataRequest) (*pb.DeleteClusterMetadataResponse, error) {
	return k.usecases.DeleteClusterMetadata(ctx, request)
}

func (k *KuberGrpcService) StoreKubeconfig(ctx context.Context, request *pb.StoreKubeconfigRequest) (*pb.StoreKubeconfigResponse, error) {
	return k.usecases.StoreKubeconfig(ctx, request)
}

func (k *KuberGrpcService) DeleteKubeconfig(ctx context.Context, request *pb.DeleteKubeconfigRequest) (*pb.DeleteKubeconfigResponse, error) {
	return k.usecases.DeleteKubeconfig(ctx, request)
}

func (k *KuberGrpcService) SetUpClusterAutoscaler(ctx context.Context, request *pb.SetUpClusterAutoscalerRequest) (*pb.SetUpClusterAutoscalerResponse, error) {
	return k.usecases.SetUpClusterAutoscaler(ctx, request)
}

func (k *KuberGrpcService) DestroyClusterAutoscaler(ctx context.Context, request *pb.DestroyClusterAutoscalerRequest) (*pb.DestroyClusterAutoscalerResponse, error) {
	return k.usecases.DestroyClusterAutoscaler(ctx, request)
}

func (k *KuberGrpcService) PatchNodes(ctx context.Context, request *pb.PatchNodesRequest) (*pb.PatchNodesResponse, error) {
	return k.usecases.PatchNodes(ctx, request)
}

func (k *KuberGrpcService) DeleteNodes(ctx context.Context, request *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	return k.usecases.DeleteNodes(ctx, request)
}
