package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/cluster"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// DeleteNodes gracefully removes nodes from specified cluster.
func (u *Usecases) DeleteNodes(ctx context.Context, request *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	logger := loggerutils.WithClusterName(cluster.Id(request.Cluster.ClusterInfo))

	logger.Info().Msgf("Deleting nodes - control nodes [%d], compute nodes[%d]", len(request.MasterNodes), len(request.WorkerNodes))
	deleter := nodes.NewDeleter(request.MasterNodes, request.WorkerNodes, request.Cluster)
	c, err := deleter.DeleteNodes()
	if err != nil {
		logger.Err(err).Msgf("Error while deleting nodes")
		return nil, fmt.Errorf("error while deleting nodes for cluster %s : %w", cluster.Id(request.Cluster.ClusterInfo), err)
	}
	logger.Info().Msgf("Nodes were successfully deleted")
	return &pb.DeleteNodesResponse{Cluster: c}, nil
}
