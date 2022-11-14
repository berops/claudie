package ansibler

import (
	"context"
	"fmt"

	"github.com/Berops/claudie/proto/pb"
)

// InstallVPN installs a Wireguard VPN on the nodes in the cluster and loadbalancers
func InstallVPN(c pb.AnsiblerServiceClient, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	res, err := c.InstallVPN(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling InstallVPN on Ansibler: %w", err)
	}
	return res, nil
}

// InstallNodeRequirements install any additional requirements nodes in the cluster have (e.g. longhorn req, ...)
func InstallNodeRequirements(c pb.AnsiblerServiceClient, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	res, err := c.InstallNodeRequirements(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling InstallNodeRequirements on Ansibler: %w", err)
	}
	return res, nil
}

// SetUpLoadbalancers ensures the nginx loadbalancer is set up correctly, with a correct DNS records
func SetUpLoadbalancers(c pb.AnsiblerServiceClient, req *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	res, err := c.SetUpLoadbalancers(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling SetUpLoadbalancers on Ansibler: %w", err)
	}
	return res, nil
}
