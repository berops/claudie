package wireguardian

import (
	"context"
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

// BuildVPN simply calls WireGuardian service client to build a VPN
func BuildVPN(c pb.WireguardianServiceClient, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	res, err := c.BuildVPN(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling BuildVPN on Wireguardian: %v", err)
	}

	log.Info().Msg("VPN was successfully built")
	return res, nil
}
