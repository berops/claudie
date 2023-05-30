package usecases

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

// DeleteKubeconfig deletes the K8s secret (in the management cluster) containing kubeconfig
// for the given K8s cluster.
func (u *Usecases) DeleteKubeconfig(ctx context.Context, request *pb.DeleteKubeconfigRequest) (*pb.DeleteKubeconfigResponse, error) {
	namespace := envs.Namespace
	// If Claudie is running outside Kubernetes.
	if namespace == "" {
		return &pb.DeleteKubeconfigResponse{}, nil
	}

	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	logger.Info().Msgf("Deleting secret containing kubeconfig")

	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, request.Cluster.ClusterInfo.Hash)

		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}

	var (
		k8sCluster = request.GetCluster()

		secretName = fmt.Sprintf("%s-%s-kubeconfig", k8sCluster.ClusterInfo.Name, k8sCluster.ClusterInfo.Hash)
	)
	// Delete the K8s secret.
	if err := kc.KubectlDeleteResource("secret", secretName, "-n", namespace); err != nil {
		logger.Warn().Msgf("Failed to remove K8s secret containing kubeconfig for this cluster: %s", err)
		return &pb.DeleteKubeconfigResponse{}, nil
	}

	logger.Info().Msgf("Secret containing kubeconfig was successfully deleted")
	return &pb.DeleteKubeconfigResponse{}, nil
}
