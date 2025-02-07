package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

// DeleteNodes gracefully removes nodes from specified cluster.
func (u *Usecases) DeleteNodes(ctx context.Context, request *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	logger := loggerutils.WithClusterName(request.Cluster.ClusterInfo.Id())

	var (
		master        []string
		worker        []string
		keepNodepools = make(map[string]struct{})
	)

	for np, deleted := range request.Nodepools {
		if nodepools.FindByName(np, request.Cluster.GetClusterInfo().GetNodePools()).GetIsControl() {
			master = append(master, deleted.Nodes...)
		} else {
			worker = append(worker, deleted.Nodes...)
		}
		if deleted.KeepNodePoolIfEmpty {
			keepNodepools[np] = struct{}{}
		}
	}

	logger.Info().Msgf("Deleting nodes - control nodes [%d], compute nodes[%d]", len(master), len(worker))
	deleter := nodes.NewDeleter(master, worker, request.Cluster, keepNodepools)
	c, err := deleter.DeleteNodes()
	if err != nil {
		logger.Err(err).Msgf("Error while deleting nodes")
		return nil, fmt.Errorf("error while deleting nodes for cluster %s : %w", request.Cluster.ClusterInfo.Id(), err)
	}
	logger.Info().Msgf("Nodes were successfully deleted")
	return &pb.DeleteNodesResponse{Cluster: c}, nil
}
