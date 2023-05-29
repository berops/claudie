package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/usecases"
)

type TerraformerGrpcService struct {
	pb.UnimplementedTerraformerServiceServer

	usecases *usecases.Usecases
}

func (t *TerraformerGrpcService) BuildInfrastructure(ctx context.Context, request *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	return t.usecases.BuildInfrastructure(request)
}

func (t *TerraformerGrpcService) DestroyInfrastructure(ctx context.Context, request *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	return t.usecases.DestroyInfrastructure(ctx, request)
}
