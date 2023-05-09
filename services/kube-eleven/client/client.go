package kubeElevenClient

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

// BuildCluster uses KubeEleven service client to deploy our cluster
func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Error().Err(err).Msgf("Error building cluster")
		return res, err
	}
	return res, nil
}
