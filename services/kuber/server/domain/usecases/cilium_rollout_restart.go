package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) CiliumRolloutRestart(request *pb.CiliumRolloutRestartRequest) (*pb.CiliumRolloutRestartResponse, error) {
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	logger.Info().Msgf("Performing a rollout of the cilium daemonset")
	kc := kubectl.Kubectl{
		Kubeconfig:        request.Cluster.Kubeconfig,
		MaxKubectlRetries: 5,
	}

	if err := kc.RolloutRestart("daemonset", "cilium", "-n kube-system"); err != nil {
		return nil, fmt.Errorf("failed to rollout restart daemonset for cilium: %w", err)
	}

	return &pb.CiliumRolloutRestartResponse{}, nil
}
