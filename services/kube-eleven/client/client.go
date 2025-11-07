package kubeElevenClient

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

// TODO: remove.

// BuildCluster uses KubeEleven service client to deploy our cluster
func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Err(err).Msgf("Error building cluster")
		return res, err
	}
	return res, nil
}

func DestroyCluster(c pb.KubeElevenServiceClient, req *pb.DestroyClusterRequest) (*pb.DestroyClusterResponse, error) {
	resp, err := c.DestroyCluster(context.Background(), req)
	if err != nil {
		log.Err(err).Msgf("Error building cluster")
		return resp, err
	}

	return resp, nil
}
