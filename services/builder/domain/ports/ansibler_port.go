package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
)

type AnsiblerPort interface {
	InstallNodeRequirements(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)
	InstallVPN(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)
	SetUpLoadbalancers(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.SetUpLBResponse, error)
	DetermineApiEndpointChange(builderCtx *builder.Context, cid string, did string, stt spec.ApiEndpointChangeState, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.DetermineApiEndpointChangeResponse, error)
	UpdateAPIEndpoint(builderCtx *builder.Context, nodepool, node string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateAPIEndpointResponse, error)
	UpdateProxyEnvsK8SServices(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) error
	UpdateProxyEnvsOnNodes(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) error
	RemoveClaudieUtilities(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) error
	InstallTeeOverride(builderCtx *builder.Context, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)

	PerformHealthCheck() error
	GetClient() pb.AnsiblerServiceClient
}
