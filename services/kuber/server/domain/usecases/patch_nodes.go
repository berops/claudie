package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// TODO: understand what this is doing and why is it doing so.
func (u *Usecases) PatchNodes(ctx context.Context, request *pb.PatchNodeTemplateRequest) (*pb.PatchNodeTemplateResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	nodePatcher := nodes.NewNodePatcher(request.Cluster)
	if err := nodePatcher.PatchProviderID(logger); err != nil {
		logger.Err(err).Msgf("Error while patching nodes")
		return nil, fmt.Errorf("error while patching nodes for %s : %w", request.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Nodes were successfully patched")
	return &pb.PatchNodeTemplateResponse{}, nil
}
