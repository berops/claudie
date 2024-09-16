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
		// If kuber deployed locally, return.
		return &pb.DeleteKubeconfigResponse{}, nil
	}
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)
	var err error
	// Log success/error message.
	defer func() {
		if err != nil {
			logger.Warn().Msgf("Failed to remove kubeconfig, secret most likely already removed : %v", err)
		} else {
			logger.Info().Msgf("Deleted kubeconfig secret")
		}
	}()

	logger.Info().Msgf("Deleting kubeconfig secret")
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() <= zerolog.InfoLevel {
		prefix := utils.GetClusterID(request.Cluster.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}

	// Save error and return as errors are ignored here.
	err = kc.KubectlDeleteResource("secret", fmt.Sprintf("%s-kubeconfig", clusterID), "-n", namespace)
	return &pb.DeleteKubeconfigResponse{}, nil
}
