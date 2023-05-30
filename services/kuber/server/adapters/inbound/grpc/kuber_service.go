package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/usecases"
)

type KuberGrpcService struct {
	pb.UnimplementedKuberServiceServer

	usecases *usecases.Usecases
}

func (k *KuberGrpcService) PatchClusterInfoConfigMap(_ context.Context, request *pb.PatchClusterInfoConfigMapRequest) (*pb.PatchClusterInfoConfigMapResponse, error) {
	return k.usecases.PatchClusterInfoConfigMap(request)
}

func (k *KuberGrpcService) SetUpStorage(ctx context.Context, request *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	return k.usecases.SetUpStorage(ctx, request)
}

func (k *KuberGrpcService) StoreLbScrapeConfig(ctx context.Context, request *pb.StoreLbScrapeConfigRequest) (*pb.StoreLbScrapeConfigResponse, error) {
	return k.usecases.StoreLbScrapeConfig(ctx, request)
}

func (k *KuberGrpcService) RemoveLbScrapeConfig(ctx context.Context, request *pb.RemoveLbScrapeConfigRequest) (*pb.RemoveLbScrapeConfigResponse, error) {
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

func (k *KuberGrpcService) PatchNodes(ctx context.Context, request *pb.PatchNodeTemplateRequest) (*pb.PatchNodeTemplateResponse, error) {
	return k.usecases.PatchNodes(ctx, request)
}
