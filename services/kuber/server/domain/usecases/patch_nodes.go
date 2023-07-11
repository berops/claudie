package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// PatchNodes uses kube API patch to set correct metadata for nodes.
func (u *Usecases) PatchNodes(ctx context.Context, request *pb.PatchNodeTemplateRequest) (*pb.PatchNodeTemplateResponse, error) {
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	logger.Info().Msgf("Patching kubernetes nodes")
	patcher := nodes.NewPatcher(request.Cluster)
	if err := patcher.PatchProviderID(logger); err != nil {
		logger.Err(err).Msgf("Error while patching nodes")
		return nil, fmt.Errorf("error while patching nodes for %s : %w", clusterID, err)
	}

	logger.Info().Msgf("Nodes were successfully patched")
	return &pb.PatchNodeTemplateResponse{}, nil
}
