package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// PatchNodes uses kube API patch to set correct metadata for nodes.
func (u *Usecases) PatchNodes(ctx context.Context, request *pb.PatchNodesRequest) (*pb.PatchNodesResponse, error) {
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	patcher := nodes.NewPatcher(request.Cluster, logger)

	if err := patcher.PatchProviderID(); err != nil {
		logger.Err(err).Msgf("Error while patching node provider ID")
		return nil, fmt.Errorf("error while patching providerID on nodes for %s : %w", clusterID, err)
	}

	if err := patcher.PatchLabels(); err != nil {
		logger.Err(err).Msgf("Error while patching node labels")
		return nil, fmt.Errorf("error while patching labels on nodes for %s : %w", clusterID, err)
	}

	if err := patcher.PatchAnnotations(); err != nil {
		logger.Err(err).Msgf("Error while patching node annotations")
		return nil, fmt.Errorf("error while patching annotations on nodes for %s : %w", clusterID, err)
	}

	if err := patcher.PatchTaints(); err != nil {
		logger.Err(err).Msgf("Error while patching node taints")
		return nil, fmt.Errorf("error while patching taints on nodes for %s : %w", clusterID, err)
	}

	logger.Info().Msgf("Nodes were successfully patched")
	return &pb.PatchNodesResponse{}, nil
}
