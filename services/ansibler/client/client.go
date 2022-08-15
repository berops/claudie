package wireguardian

import (
	"context"
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

// InstallVPN installs a Wireguard VPN on the nodes in the cluster and loadbalancers
func InstallVPN(c pb.AnsiblerServiceClient, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	res, err := c.InstallVPN(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling InstallVPN on Ansibler: %v", err)
	}
	log.Info().Msg("VPN was successfully installed")
	return res, nil
}

// InstallNodeRequirements install any additional requirements nodes in the cluster have (e.g. longhorn req, ...)
func InstallNodeRequirements(c pb.AnsiblerServiceClient, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	res, err := c.InstallNodeRequirements(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling InstallNodeRequirements on Ansibler: %v", err)
	}
	log.Info().Msg("Node requirements were successfully installed")
	return res, nil
}

// SetUpLoadbalancers ensures the nginx loadbalancer is set up correctly, with a correct DNS records
func SetUpLoadbalancers(c pb.AnsiblerServiceClient, req *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	res, err := c.SetUpLoadbalancers(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling SetUpLoadbalancers on Ansibler: %v", err)
	}
	log.Info().Msg("Loadbalancers were successfully set up")
	return res, nil
}
