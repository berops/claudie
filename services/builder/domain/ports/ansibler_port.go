package ports

import (
	"github.com/berops/claudie/proto/pb"
	builder "github.com/berops/claudie/services/builder/internal"
)

type AnsiblerPort interface {
	InstallNodeRequirements(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)
	InstallVPN(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)
	SetUpLoadbalancers(builderCtx *builder.Context, apiEndpoint string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.SetUpLBResponse, error)
	TeardownLoadBalancers(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.TeardownLBResponse, error)
	UpdateAPIEndpoint(builderCtx *builder.Context, nodepool, node string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateAPIEndpointResponse, error)
	UpdateNoProxyEnvsInKubernetes(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateNoProxyEnvsInKubernetesResponse, error)
	UpdateProxyEnvsOnNodes(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateProxyEnvsOnNodesResponse, error)
	RemoveClaudieUtilities(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.RemoveClaudieUtilitiesResponse, error)

	PerformHealthCheck() error
	GetClient() pb.AnsiblerServiceClient
}
