package usecases

import (
	"fmt"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	kube_eleven "github.com/berops/claudie/services/kube-eleven/server/domain/utils/kube-eleven"
)

func (u *Usecases) DestroyCluster(req *pb.DestroyClusterRequest) (*pb.DestroyClusterResponse, error) {
	logger := utils.CreateLoggerWithProjectAndClusterName(req.ProjectName, utils.GetClusterID(req.Current.ClusterInfo))

	logger.Info().Msgf("Destroying kubernetes cluster")

	k := kube_eleven.KubeEleven{
		K8sCluster:        req.Current,
		LBClusters:        req.CurrentLbs,
		SpawnProcessLimit: u.SpawnProcessLimit,
	}

	if err := k.DestroyCluster(); err != nil {
		logger.Error().Msgf("Error while destroying cluster: %s", err)
		return nil, fmt.Errorf("error while destroying cluster %s for project %s: %w", req.Current.ClusterInfo.Name, req.ProjectName, err)
	}

	logger.Info().Msgf("Kubernetes cluster was successfully destroyed")
	return &pb.DestroyClusterResponse{Current: req.Current, CurrentLbs: req.CurrentLbs}, nil
}
