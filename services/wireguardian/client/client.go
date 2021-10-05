package wireguardian

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
)

func init() {
	// intialize logging framework
	utils.InitLog("wireguardian")
}

// BuildVPN simply calls WireGuardian service client to build a VPN
func BuildVPN(c pb.WireguardianServiceClient, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	res, err := c.BuildVPN(context.Background(), req)
	if err != nil {
		log.Fatal().Msg("Failed to build VPN")
		return nil, err
	}

	log.Info().Msg("VPN was successfully built")
	return res, nil
}
