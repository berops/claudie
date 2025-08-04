package usecases

import (
	"context"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) GpuOperatorRolloutRestart(ctx context.Context, req *pb.GpuOperatorRolloutRestartRequest) (*pb.GpuOperatorRolloutRestartResponse, error) {
	clusterID := req.Cluster.ClusterInfo.Id()
	logger := loggerutils.WithClusterName(clusterID)

	logger.Info().Msgf("Performing a rollout restart of the NVIDIA GPU Operator toolking daemon set, if present")
	kc := kubectl.Kubectl{
		Kubeconfig:        req.Cluster.Kubeconfig,
		MaxKubectlRetries: 2,
	}

	// Based on the issue: https://github.com/NVIDIA/gpu-operator/issues/598
	// This rollout restart is needed if the NVIDIA GPU operator is deployed which overwrites the
	// containerd config under `/etc/containerd/config.toml`. Any change to the cluster results in kubeone
	// overwritting the config to its values, but nvidia also overwrites it so that GPU's can be used within the cluster.
	// To add/remove nodes in the cluster where the GPU operator is deployed we make the rollout restart as part of the
	// workflow.
	if err := kc.RolloutRestart("daemonset", "nvidia-container-toolkit-daemonset", "-n gpu-operator"); err != nil {
		logger.Warn().Msgf("Failed to rollout restart NVIDIA toolkit daemon set: %v, assuming the GPU operator is not deployed, continuing", err)
	}

	return &pb.GpuOperatorRolloutRestartResponse{}, nil
}
