package wireguardian

import (
	"context"
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

// RunAnsible executes all tasks run in ansible (wireguard, LB configs, ...)
func RunAnsible(c pb.WireguardianServiceClient, req *pb.RunAnsibleRequest) (*pb.RunAnsibleResponse, error) {
	res, err := c.RunAnsible(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling BuildVPN on Wireguardian: %v", err)
	}

	log.Info().Msg("VPN was successfully built")
	return res, nil
}
