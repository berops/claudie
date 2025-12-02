package outbound

import (
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	ansibler "github.com/berops/claudie/services/ansibler/client"
	builder "github.com/berops/claudie/services/builder/internal"

	"google.golang.org/grpc"
)

type AnsiblerConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the ansibler microservice.
func (a *AnsiblerConnector) Connect() error {
	connection, err := grpcutils.GrpcDialWithRetryAndBackoff("ansibler", envs.AnsiblerURL)
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
func (a *AnsiblerConnector) SetUpLoadbalancers(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.SetUpLBResponse, error) {
	return ansibler.SetUpLoadbalancers(ansiblerGrpcClient,
		&pb.SetUpLBRequest{
			Desired:     builderCtx.DesiredCluster,
			CurrentLbs:  builderCtx.CurrentLoadbalancers,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// DetermineApiEndpointChange determines if the api endpoint of the k8s cluster should be moved based on the changes to the
// loadbalancer infrastructure.
func (a *AnsiblerConnector) DetermineApiEndpointChange(builderCtx *builder.Context, cid string, did string, stt spec.ApiEndpointChangeState, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.DetermineApiEndpointChangeResponse, error) {
	return ansibler.DetermineApiEndpointChange(ansiblerGrpcClient,
		&pb.DetermineApiEndpointChangeRequest{
			Current:           builderCtx.CurrentCluster,
			CurrentLbs:        builderCtx.CurrentLoadbalancers,
			ProjectName:       builderCtx.ProjectName,
			State:             stt,
			CurrentEndpointId: cid,
			DesiredEndpointId: did,
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

// UpdateNoProxyEnvsInKubernetes updates NO_PROXY and no_proxy envs in kube-proxy and static pods.
func (a *AnsiblerConnector) UpdateProxyEnvsK8SServices(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) error {
	_, err := ansibler.UpdateProxyEnvsK8SServices(ansiblerGrpcClient, &pb.UpdateProxyEnvsK8SServicesRequest{
		Current:     builderCtx.CurrentCluster,
		Desired:     builderCtx.DesiredCluster,
		ProxyEnvs:   builderCtx.ProxyEnvs,
		ProjectName: builderCtx.ProjectName,
	})
	return err
}

// UpdateProxyEnvsOnNodes updates proxy envs on all nodes of the cluster.
func (a *AnsiblerConnector) UpdateProxyEnvsOnNodes(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) error {
	_, err := ansibler.UpdateProxyEnvsOnNodes(ansiblerGrpcClient, &pb.UpdateProxyEnvsOnNodesRequest{
		Desired:     builderCtx.DesiredCluster,
		ProxyEnvs:   builderCtx.ProxyEnvs,
		ProjectName: builderCtx.ProjectName,
	})
	return err
}

// RemoveClaudieUtilities removes claudie installed utilities from the nodes of the cluster.
func (a *AnsiblerConnector) RemoveClaudieUtilities(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) error {
	_, err := ansibler.RemoveClaudieUtilities(ansiblerGrpcClient, &pb.RemoveClaudieUtilitiesRequest{
		Current:     builderCtx.CurrentCluster,
		CurrentLbs:  builderCtx.CurrentLoadbalancers,
		ProjectName: builderCtx.ProjectName,
	})
	return err
}

// InstallTeeOverride installs node requirements on all nodes.
func (a *AnsiblerConnector) InstallTeeOverride(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error) {
	return ansibler.InstallTeeOverride(ansiblerGrpcClient,
		&pb.InstallRequest{Desired: builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// Disconnect closes the underlying gRPC connection to ansibler microservice
func (a *AnsiblerConnector) Disconnect() {
	grpcutils.CloseClientConnection(a.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to ansibler microservice
func (a *AnsiblerConnector) PerformHealthCheck() error {
	if err := grpcutils.IsConnectionReady(a.Connection); err == nil {
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
