package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// DeleteNodes gracefully removes nodes from specified cluster.
func (u *Usecases) DeleteNodes(ctx context.Context, request *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	logger.Info().Msgf("Deleting nodes - control nodes [%d], compute nodes[%d]", len(request.MasterNodes), len(request.WorkerNodes))
	deleter := nodes.NewDeleter(request.MasterNodes, request.WorkerNodes, request.Cluster)
	cluster, err := deleter.DeleteNodes()
	if err != nil {
		logger.Err(err).Msgf("Error while deleting nodes")
		return nil, fmt.Errorf("error while deleting nodes for cluster %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Nodes were successfully deleted")
	return &pb.DeleteNodesResponse{Cluster: cluster}, nil
}
