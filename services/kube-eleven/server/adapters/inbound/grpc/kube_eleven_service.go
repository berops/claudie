package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kube-eleven/server/domain/usecases"
)

type KubeElevenGrpcService struct {
	pb.UnimplementedKubeElevenServiceServer

	usecases *usecases.Usecases
}

func (k *KubeElevenGrpcService) BuildCluster(_ context.Context, request *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	return k.usecases.BuildCluster(request)
}
