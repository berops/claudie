package ports

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

type AnsiblerPort interface {
	InstallNodeRequirements(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)
	InstallVPN(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.InstallResponse, error)
	SetUpLoadbalancers(builderCtx *utils.BuilderContext, apiEndpoint string, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.SetUpLBResponse, error)
	TeardownLoadBalancers(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.TeardownLBResponse, error)
	UpdateAPIEndpoint(builderCtx *utils.BuilderContext, ansiblerGrpcClient pb.AnsiblerServiceClient) (*pb.UpdateAPIEndpointResponse, error)
	PerformHealthCheck() error
	GetClient() pb.AnsiblerServiceClient
}
