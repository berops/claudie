package usecases

import (
	"context"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
)

func (u *Usecases) DeleteNodes(ctx context.Context, request *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	logger.Info().Msgf("Deleting nodes - %d control nodes , %d compute nodes", len(request.MasterNodes), len(request.WorkerNodes))

	deleter := nodes.NewNodeDeleter(request.MasterNodes, request.WorkerNodes, request.Cluster)
	cluster, err := deleter.DeleteNodes()
	if err != nil {
		logger.Err(err).Msgf("Error while deleting nodes")
		return &pb.DeleteNodesResponse{}, err
	}

	logger.Info().Msgf("Nodes were successfully deleted")
	return &pb.DeleteNodesResponse{Cluster: cluster}, nil
}
