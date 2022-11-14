package kubeElevenClient

import (
	"context"

	"github.com/Berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
)

// BuildCluster uses KubeEleven service client to deploy our cluster
func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Error().Msgf("Error building cluster: %v", err)
		return res, err
	}
	return res, nil
}
