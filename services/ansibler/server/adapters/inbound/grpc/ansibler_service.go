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

func (a *AnsiblerGrpcService) UpdateProxyEnvsK8SServices(_ context.Context, request *pb.UpdateProxyEnvsK8SServicesRequest) (*pb.UpdateProxyEnvsK8SServicesResponse, error) {
	return a.usecases.UpdateProxyEnvsK8sServices(request)
}

func (a *AnsiblerGrpcService) UpdateProxyEnvsOnNodes(_ context.Context, request *pb.UpdateProxyEnvsOnNodesRequest) (*pb.UpdateProxyEnvsOnNodesResponse, error) {
	return a.usecases.UpdateProxyEnvsOnNodes(request)
}

func (a *AnsiblerGrpcService) InstallNodeRequirements(_ context.Context, request *pb.InstallRequest) (*pb.InstallResponse, error) {
	panic("")
}

func (a *AnsiblerGrpcService) InstallVPN(_ context.Context, request *pb.InstallRequest) (*pb.InstallResponse, error) {
	panic("")
}

func (a *AnsiblerGrpcService) SetUpLoadbalancers(_ context.Context, request *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	panic("")
}

func (a *AnsiblerGrpcService) DetermineApiEndpointChange(ctx context.Context, request *pb.DetermineApiEndpointChangeRequest) (*pb.DetermineApiEndpointChangeResponse, error) {
	panic("")
}
