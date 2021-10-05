package kubeEleven

import (
	"context"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

func init() {
	// intialize logging framework
	utils.InitLog("kubeEleven")
}

// BuildCluster uses KubeEleven service client to deploy our cluster
func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Error().Msgf("Error building cluster: %v", err)
		return nil, err
	}
	log.Info().Msg("Clusters were successfully built")
	return res, nil
}
