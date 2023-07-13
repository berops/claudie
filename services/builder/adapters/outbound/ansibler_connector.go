package outbound

import (
	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	ansibler "github.com/berops/claudie/services/ansibler/client"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	"google.golang.org/grpc"
)

type AnsiblerConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the context-box microservice
func (a *AnsiblerConnector) Connect() error {
	connection, err := cutils.GrpcDialWithRetryAndBackoff("ansibler", envs.AnsiblerURL)
	if err != nil {
		return err
	}
	a.Connection = connection

	return nil
}

func (a *AnsiblerConnector) InstallNodeRequirements(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error) {
	return ansibler.InstallNodeRequirements(ansiblerGrpcClient,
		&pb.InstallRequest{Desired: builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

func (a *AnsiblerConnector) InstallVPN(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error) {
	return ansibler.InstallVPN(ansiblerGrpcClient,
		&pb.InstallRequest{
			Desired:     builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

func (a *AnsiblerConnector) SetUpLoadbalancers(builderCtx *utils.BuilderContext, apiEndpoint string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.SetUpLBResponse, error) {
	return ansibler.SetUpLoadbalancers(ansiblerGrpcClient,
		&pb.SetUpLBRequest{
			Desired:             builderCtx.DesiredCluster,
			CurrentLbs:          builderCtx.CurrentLoadbalancers,
			DesiredLbs:          builderCtx.DesiredLoadbalancers,
			PreviousAPIEndpoint: apiEndpoint,
			ProjectName:         builderCtx.ProjectName,
			FirstRun:            builderCtx.CurrentCluster == nil,
		})
}

func (a *AnsiblerConnector) TeardownLoadBalancers(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.TeardownLBResponse, error) {
	return ansibler.TeardownLoadBalancers(ansiblerGrpcClient,
		&pb.TeardownLBRequest{
			Desired:     builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			DeletedLbs:  builderCtx.DeletedLoadBalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

func (a *AnsiblerConnector) UpdateAPIEndpoint(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateAPIEndpointResponse, error) {
	return ansibler.UpdateAPIEndpoint(ansiblerGrpcClient, &pb.UpdateAPIEndpointRequest{
		Current:     builderCtx.CurrentCluster,
		Desired:     builderCtx.DesiredCluster,
		ProjectName: builderCtx.ProjectName,
	})
}

// Disconnect closes the underlying gRPC connection to context-box microservice
func (a *AnsiblerConnector) Disconnect() {
	cutils.CloseClientConnection(a.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to context-box microservice
func (a *AnsiblerConnector) PerformHealthCheck() error {
	return cutils.IsConnectionReady(a.Connection)
}
func (a *AnsiblerConnector) GetClient() pb.AnsiblerServiceClient {
	return pb.NewAnsiblerServiceClient(a.Connection)
}
