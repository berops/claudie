package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/domain/usecases"
)

type AnsiblerGrpcService struct {
	pb.UnimplementedAnsiblerServiceServer

	usecases usecases.Usecases
}

func (a *AnsiblerGrpcService) UpdateAPIEndpoint(_ context.Context, request *pb.UpdateAPIEndpointRequest) (*pb.UpdateAPIEndpointResponse, error) {
	return a.usecases.UpdateAPIEndpoint(request)
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

func (a *AnsiblerGrpcService) TeardownLoadBalancers(ctx context.Context, request *pb.TeardownLBRequest) (*pb.TeardownLBResponse, error) {
	return a.usecases.TeardownLoadBalancers(ctx, request)
}
