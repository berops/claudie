package grpc

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

type KubeElevenGrpcService struct {
	pb.UnimplementedKubeElevenServiceServer
}

// BuildCluster builds all cluster defined in the desired state
func (k *KubeElevenGrpcService) BuildCluster(_ context.Context, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	log.Info().Msgf("Building kubernetes cluster %s project %s", req.Desired.ClusterInfo.Name, req.ProjectName)
	ke := kubeEleven.KubeEleven{
		K8sCluster: req.Desired,
		LBClusters: req.DesiredLbs,
	}

	if err := ke.BuildCluster(); err != nil {
		return nil, fmt.Errorf("error while building cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	log.Info().Msgf("Kubernetes cluster %s project %s was successfully build", req.Desired.ClusterInfo.Name, req.ProjectName)
	return &pb.BuildClusterResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}
