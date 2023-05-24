package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	kube_eleven "github.com/berops/claudie/services/kube-eleven/server/domain/utils/kube-eleven"
)

// BuildCluster builds all cluster defined in the desired state
func (u *Usecases) BuildCluster(req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	logger := utils.CreateLoggerWithProjectAndClusterName(req.ProjectName, utils.GetClusterID(req.Desired.ClusterInfo))

	logger.Info().Msgf("Building kubernetes cluster")

	k := kube_eleven.KubeEleven{
		K8sCluster: req.Desired,
		LBClusters: req.DesiredLbs,
	}

	if err := k.BuildCluster(); err != nil {
		logger.Err(err).Msgf("Error while building a cluster")
		return nil, fmt.Errorf("error while building cluster %s for project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	logger.Info().Msgf("Kubernetes cluster was successfully build")
	return &pb.BuildClusterResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}
