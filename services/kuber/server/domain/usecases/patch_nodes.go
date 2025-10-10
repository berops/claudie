package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// PatchNodes uses kube API patch to set correct metadata for nodes.
func (u *Usecases) PatchNodes(ctx context.Context, request *pb.PatchNodesRequest) (*pb.PatchNodesResponse, error) {
	clusterID := request.Cluster.ClusterInfo.Id()
	logger := loggerutils.WithClusterName(clusterID)

	patcher := nodes.NewPatcher(request.Cluster, request.ToRemove, logger)

	if err := patcher.Wait(); err != nil {
		logger.Err(err).Msgf("Error while patching nodes")
		return nil, fmt.Errorf("error while patching nodes for %s : %w", clusterID, err)
	}

	logger.Info().Msgf("Nodes were successfully patched")
	return &pb.PatchNodesResponse{}, nil
}
