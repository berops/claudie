package outbound

import (
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	ansibler "github.com/berops/claudie/services/ansibler/client"
	builder "github.com/berops/claudie/services/builder/internal"

	"google.golang.org/grpc"
)

type AnsiblerConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the ansibler microservice.
func (a *AnsiblerConnector) Connect() error {
	connection, err := utils.GrpcDialWithRetryAndBackoff("ansibler", envs.AnsiblerURL)
	if err != nil {
		return err
	}

	a.Connection = connection

	return nil
}

// InstallNodeRequirements installs node requirements on all nodes.
func (a *AnsiblerConnector) InstallNodeRequirements(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error) {
	return ansibler.InstallNodeRequirements(ansiblerGrpcClient,
		&pb.InstallRequest{Desired: builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// InstallVPN installs VPN on all nodes of the infrastructure.
func (a *AnsiblerConnector) InstallVPN(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error) {
	return ansibler.InstallVPN(ansiblerGrpcClient,
		&pb.InstallRequest{
			Desired:     builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// SetUpLoadbalancers configures loadbalancers for the infrastructure.
func (a *AnsiblerConnector) SetUpLoadbalancers(builderCtx *builder.Context, apiEndpoint string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.SetUpLBResponse, error) {
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

// TeardownLoadBalancers destroys loadbalancers for the infrastructure.
func (a *AnsiblerConnector) TeardownLoadBalancers(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.TeardownLBResponse, error) {
	return ansibler.TeardownLoadBalancers(ansiblerGrpcClient,
		&pb.TeardownLBRequest{
			Desired:     builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			DeletedLbs:  builderCtx.DeletedLoadBalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// UpdateAPIEndpoint updates kube API endpoint of the cluster.
func (a *AnsiblerConnector) UpdateAPIEndpoint(builderCtx *builder.Context, nodepool, node string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateAPIEndpointResponse, error) {
	return ansibler.UpdateAPIEndpoint(ansiblerGrpcClient, &pb.UpdateAPIEndpointRequest{
		Endpoint:    &pb.UpdateAPIEndpointRequest_Endpoint{Nodepool: nodepool, Node: node},
		Current:     builderCtx.CurrentCluster,
		ProjectName: builderCtx.ProjectName,
	})
}

// UpdateAPIEndpoint updates kube API endpoint of the cluster.
func (a *AnsiblerConnector) UpdateNoProxyEnvs(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateNoProxyEnvsResponse, error) {
	return ansibler.UpdateNoProxyEnvs(ansiblerGrpcClient, &pb.UpdateNoProxyEnvsRequest{
		Current:     builderCtx.CurrentCluster,
		Desired:     builderCtx.DesiredCluster,
		DesiredLbs:  builderCtx.DesiredLoadbalancers,
		ProjectName: builderCtx.ProjectName,
	})
}

// RemoveClaudieUtilities removes claudie installed utilities from the nodes of the cluster.
func (a *AnsiblerConnector) RemoveClaudieUtilities(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.RemoveClaudieUtilitiesResponse, error) {
	return ansibler.RemoveClaudieUtilities(ansiblerGrpcClient, &pb.RemoveClaudieUtilitiesRequest{
		Current:     builderCtx.CurrentCluster,
		CurrentLbs:  builderCtx.CurrentLoadbalancers,
		ProjectName: builderCtx.ProjectName,
	})
}

// Disconnect closes the underlying gRPC connection to ansibler microservice
func (a *AnsiblerConnector) Disconnect() {
	utils.CloseClientConnection(a.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to ansibler microservice
func (a *AnsiblerConnector) PerformHealthCheck() error {
	if err := utils.IsConnectionReady(a.Connection); err == nil {
		return nil
	} else {
		a.Connection.Connect()
		return err
	}
}

// GetClient returns a ansibler gRPC client.
func (a *AnsiblerConnector) GetClient() pb.AnsiblerServiceClient {
	return pb.NewAnsiblerServiceClient(a.Connection)
}
