package ports

import (
	"github.com/berops/claudie/proto/pb"
	builder "github.com/berops/claudie/services/builder/internal"
)

type TerraformerPort interface {
	BuildInfrastructure(builderCtx *builder.Context, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.BuildInfrastructureResponse, error)
	DestroyInfrastructure(builderCtx *builder.Context, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.DestroyInfrastructureResponse, error)

	PerformHealthCheck() error
	GetClient() pb.TerraformerServiceClient
}
