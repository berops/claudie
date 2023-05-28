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
