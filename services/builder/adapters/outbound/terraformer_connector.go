package outbound

import (
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	builder "github.com/berops/claudie/services/builder/internal"
	terraformer "github.com/berops/claudie/services/terraformer/client"

	"google.golang.org/grpc"
)

type TerraformerConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the terraformer microservice.
func (t *TerraformerConnector) Connect() error {
	connection, err := grpcutils.GrpcDialWithRetryAndBackoff("terraformer", envs.TerraformerURL)
	if err != nil {
		return err
	}

	t.Connection = connection

	return nil
}

// BuildInfrastructure builds/reconciles the infrastructure for given k8s cluster via terraformer.
func (t *TerraformerConnector) BuildInfrastructure(builderCtx *builder.Context, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.BuildInfrastructureResponse, error) {
	return terraformer.BuildInfrastructure(terraformerGrpcClient,
		&pb.BuildInfrastructureRequest{
			Current:     builderCtx.CurrentCluster,
			Desired:     builderCtx.DesiredCluster,
			CurrentLbs:  builderCtx.CurrentLoadbalancers,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
			Options:     builderCtx.Options,
		})
}

// DestroyInfrastructure destroys the infrastructure for given k8s cluster via terraformer.
func (t *TerraformerConnector) DestroyInfrastructure(builderCtx *builder.Context, terraformerGrpcClient pb.TerraformerServiceClient) (*pb.DestroyInfrastructureResponse, error) {
	return terraformer.DestroyInfrastructure(terraformerGrpcClient, &pb.DestroyInfrastructureRequest{
		ProjectName: builderCtx.ProjectName,
		Current:     builderCtx.CurrentCluster,
		CurrentLbs:  builderCtx.CurrentLoadbalancers,
	})
}

// Disconnect closes the underlying gRPC connection to terraformer microservice.
func (t *TerraformerConnector) Disconnect() {
	grpcutils.CloseClientConnection(t.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to terraformer microservice.
func (t *TerraformerConnector) PerformHealthCheck() error {
	if err := grpcutils.IsConnectionReady(t.Connection); err == nil {
		return nil
	} else {
		t.Connection.Connect()
		return err
	}
}

// GetClient returns a terraformer gRPC client.
func (t *TerraformerConnector) GetClient() pb.TerraformerServiceClient {
	return pb.NewTerraformerServiceClient(t.Connection)
}
