package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

type TerraformerPort interface {
	BuildInfrastructure(builderCtx *utils.BuilderContext, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.BuildInfrastructureResponse, error)
	DestroyInfrastructure(builderCtx *utils.BuilderContext, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.DestroyInfrastructureResponse, error)

	PerformHealthCheck() error
	GetClient() pb.TerraformerServiceClient
}
