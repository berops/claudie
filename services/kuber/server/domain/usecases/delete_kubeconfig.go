package usecases

import (
	"context"
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// DeleteKubeconfig deletes the K8s secret (in the management cluster) containing kubeconfig
// for the given K8s cluster.
func (u *Usecases) DeleteKubeconfig(ctx context.Context, request *pb.DeleteKubeconfigRequest) (*pb.DeleteKubeconfigResponse, error) {
	namespace := envs.Namespace
	if namespace == "" {
		return &pb.DeleteKubeconfigResponse{}, nil
	}
	cluster := request.GetCluster()
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	logger.Info().Msgf("Deleting kubeconfig secret")
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := utils.GetClusterID(request.Cluster.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	secretName := fmt.Sprintf("%s-%s-kubeconfig", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)

	if err := kc.KubectlDeleteResource("secret", secretName, "-n", namespace); err != nil {
		logger.Warn().Msgf("Failed to remove kubeconfig: %s", err)
		return &pb.DeleteKubeconfigResponse{}, nil
	}

	logger.Info().Msgf("Deleted kubeconfig secret")
	return &pb.DeleteKubeconfigResponse{}, nil
}
