package usecases

import (
	"context"

	"github.com/berops/claudie/proto/pb"
)

// TeardownLoadBalancers correctly destroys loadbalancers by selecting the new ApiServer endpoint
func (a *Usecases) TeardownLoadBalancers(ctx context.Context, request *pb.TeardownLBRequest) (*pb.TeardownLBResponse, error) {

	// TODO: implement

	return &pb.TeardownLBResponse{}, nil
}
