package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/domain/usecases"
)

type AnsiblerGrpcService struct {
	pb.UnimplementedAnsiblerServiceServer

	usecases *usecases.Usecases
}

func (a *AnsiblerGrpcService) RemoveClaudieUtilities(_ context.Context, request *pb.RemoveClaudieUtilitiesRequest) (*pb.RemoveClaudieUtilitiesResponse, error) {
	return a.usecases.RemoveUtilities(request)
}

func (a *AnsiblerGrpcService) UpdateAPIEndpoint(_ context.Context, request *pb.UpdateAPIEndpointRequest) (*pb.UpdateAPIEndpointResponse, error) {
	return a.usecases.UpdateAPIEndpoint(request)
}

func (a *AnsiblerGrpcService) UpdateNoProxyEnvsInKubernetes(_ context.Context, request *pb.UpdateNoProxyEnvsInKubernetesRequest) (*pb.UpdateNoProxyEnvsInKubernetesResponse, error) {
	return a.usecases.UpdateNoProxyEnvsInKubernetes(request)
}

func (a *AnsiblerGrpcService) UpdateProxyEnvsOnNodes(_ context.Context, request *pb.UpdateProxyEnvsOnNodesRequest) (*pb.UpdateProxyEnvsOnNodesResponse, error) {
	return a.usecases.UpdateProxyEnvsOnNodes(request)
}

func (a *AnsiblerGrpcService) InstallNodeRequirements(_ context.Context, request *pb.InstallRequest) (*pb.InstallResponse, error) {
	return a.usecases.InstallNodeRequirements(request)
}

func (a *AnsiblerGrpcService) InstallVPN(_ context.Context, request *pb.InstallRequest) (*pb.InstallResponse, error) {
	return a.usecases.InstallVPN(request)
}

func (a *AnsiblerGrpcService) SetUpLoadbalancers(_ context.Context, request *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	return a.usecases.SetUpLoadbalancers(request)
}

func (a *AnsiblerGrpcService) TeardownApiEndpointLoadbalancer(ctx context.Context, request *pb.TeardownRequest) (*pb.TeardownResponse, error) {
	return a.usecases.TeardownApiEndpointLoadbalancer(ctx, request)
}
