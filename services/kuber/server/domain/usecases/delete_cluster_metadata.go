package usecases

import (
	"context"
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
)

// DeleteClusterMetadata deletes the K8s secret (from the management cluster) containing cluster
// metadata for the given K8s cluster.
func (u *Usecases) DeleteClusterMetadata(ctx context.Context, request *pb.DeleteClusterMetadataRequest) (*pb.DeleteClusterMetadataResponse, error) {
	namespace := envs.Namespace
	if namespace == "" {
		// If kuber deployed locally, return.
		return &pb.DeleteClusterMetadataResponse{}, nil
	}
	id := request.Cluster.ClusterInfo.Id()

	logger := loggerutils.WithClusterName(id)
	var err error
	// Log success/error message.
	defer func() {
		if err != nil {
			logger.Warn().Msgf("Failed to remove cluster metadata, secret most likely already removed : %v", err)
		} else {
			logger.Info().Msgf("Deleted cluster metadata secret")
		}
	}()

	logger.Info().Msgf("Deleting cluster metadata secret")
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	kc.Stdout = comm.GetStdOut(id)
	kc.Stderr = comm.GetStdErr(id)

	// Save error and return as errors are ignored here.
	err = kc.KubectlDeleteResource("secret", fmt.Sprintf("%s-metadata", id), "-n", namespace)
	return &pb.DeleteClusterMetadataResponse{}, nil
}
