package outbound

import (
	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	terraformer "github.com/berops/claudie/services/terraformer/client"
	"google.golang.org/grpc"
)

type TerraformerConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the context-box microservice
func (t *TerraformerConnector) Connect() error {
	connection, err := cutils.GrpcDialWithRetryAndBackoff("terraformer", envs.TerraformerURL)
	if err != nil {
		return err
	}
	t.Connection = connection

	return nil
}

func (t *TerraformerConnector) BuildInfrastructure(builderCtx *utils.BuilderContext, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.BuildInfrastructureResponse, error) {
	return terraformer.BuildInfrastructure(terraformerGrpcClient,
		&pb.BuildInfrastructureRequest{
			Current:     builderCtx.CurrentCluster,
			Desired:     builderCtx.DesiredCluster,
			CurrentLbs:  builderCtx.CurrentLoadbalancers,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

func (t *TerraformerConnector) DestroyInfrastructure(builderCtx *utils.BuilderContext, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.DestroyInfrastructureResponse, error) {
	return terraformer.DestroyInfrastructure(terraformerGrpcClient, &pb.DestroyInfrastructureRequest{
		ProjectName: builderCtx.ProjectName,
		Current:     builderCtx.CurrentCluster,
		CurrentLbs:  builderCtx.CurrentLoadbalancers,
	})
}

// Disconnect closes the underlying gRPC connection to context-box microservice
func (t *TerraformerConnector) Disconnect() {
	cutils.CloseClientConnection(t.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to context-box microservice
func (t *TerraformerConnector) PerformHealthCheck() error {
	return cutils.IsConnectionReady(t.Connection)
}
func (t *TerraformerConnector) GetClient() pb.TerraformerServiceClient {
	return pb.NewTerraformerServiceClient(t.Connection)
}
