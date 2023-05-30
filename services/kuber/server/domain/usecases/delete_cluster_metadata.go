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

// DeleteClusterMetadata deletes the K8s secret (from the management cluster) containing cluster
// metadata for the given K8s cluster.
func (u *Usecases) DeleteClusterMetadata(ctx context.Context, request *pb.DeleteClusterMetadataRequest) (*pb.DeleteClusterMetadataResponse, error) {
	// If Claudie is running outside Kubernetes, then do nothing,
	// since the K8s secret was not created.
	namespace := envs.Namespace
	if namespace == "" {
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))
	logger.Info().Msgf("Deleting secret (from management cluster) containing cluster-metadata")

	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, request.Cluster.ClusterInfo.Hash)

		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}

	// Delete the K8s secret
	secretName := fmt.Sprintf("%s-%s-metadata", request.Cluster.ClusterInfo.Name, request.Cluster.ClusterInfo.Hash)
	if err := kc.KubectlDeleteResource("secret", secretName, "-n", namespace); err != nil {
		logger.Warn().Msgf("Failed to remove cluster metadata: %s", err)
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger.Info().Msgf("Deleted secret (from management cluster) containing cluster-metadata")

	return &pb.DeleteClusterMetadataResponse{}, nil
}
