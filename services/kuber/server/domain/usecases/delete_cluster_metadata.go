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

// DeleteClusterMetadata deletes the K8s secret (from the management cluster) containing cluster
// metadata for the given K8s cluster.
func (u *Usecases) DeleteClusterMetadata(ctx context.Context, request *pb.DeleteClusterMetadataRequest) (*pb.DeleteClusterMetadataResponse, error) {
	namespace := envs.Namespace
	if namespace == "" {
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))
	logger.Info().Msgf("Deleting cluster metadata secret")

	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := utils.GetClusterID(request.Cluster.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	secretName := fmt.Sprintf("%s-%s-metadata", request.Cluster.ClusterInfo.Name, request.Cluster.ClusterInfo.Hash)
	if err := kc.KubectlDeleteResource("secret", secretName, "-n", namespace); err != nil {
		logger.Warn().Msgf("Failed to remove cluster metadata: %s", err)
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger.Info().Msgf("Deleted cluster metadata secret")
	return &pb.DeleteClusterMetadataResponse{}, nil
}
