package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
	kubeEleven "github.com/berops/claudie/services/kube-eleven/server/kube-eleven"
)

// BuildCluster builds all cluster defined in the desired state
func (u *Usecases) BuildCluster(req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	log.Info().Msgf("Building Kubernetes cluster %s for project %s", req.Desired.ClusterInfo.Name, req.ProjectName)

	k := kubeEleven.KubeEleven{
		K8sCluster: req.Desired,
		LBClusters: req.DesiredLbs,
	}

	if err := k.BuildCluster(); err != nil {
		return nil, fmt.Errorf("error while building cluster %s for project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	log.Info().Msgf("Kubernetes cluster %s for project %s was successfully built", req.Desired.ClusterInfo.Name, req.ProjectName)
	return &pb.BuildClusterResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}
