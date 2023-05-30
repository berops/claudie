package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/autoscaler"
	"github.com/berops/claudie/services/kuber/server/nodes"
)

const (
	outputDir = "services/kuber/server/clusters"
)

type server struct {
	pb.UnimplementedKuberServiceServer
}

func (s *server) DeleteNodes(ctx context.Context, req *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

	logger.Info().Msgf("Deleting nodes - control nodes [%d], compute nodes[%d]", len(req.MasterNodes), len(req.WorkerNodes))
	deleter := nodes.NewDeleter(req.MasterNodes, req.WorkerNodes, req.Cluster)
	cluster, err := deleter.DeleteNodes()
	if err != nil {
		logger.Err(err).Msgf("Error while deleting nodes")
		return &pb.DeleteNodesResponse{}, err
	}
	logger.Info().Msgf("Nodes were successfully deleted")
	return &pb.DeleteNodesResponse{Cluster: cluster}, nil
}

func (s *server) PatchNodes(ctx context.Context, req *pb.PatchNodeTemplateRequest) (*pb.PatchNodeTemplateResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

	patcher := nodes.NewPatcher(req.Cluster)
	if err := patcher.PatchProviderID(logger); err != nil {
		logger.Err(err).Msgf("Error while patching nodes")
		return nil, fmt.Errorf("error while patching nodes for %s : %w", req.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Nodes were successfully patched")
	return &pb.PatchNodeTemplateResponse{}, nil
}

func (s *server) DestroyClusterAutoscaler(ctx context.Context, req *pb.DestroyClusterAutoscalerRequest) (*pb.DestroyClusterAutoscalerResponse, error) {
	// Create output dir
	clusterID := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterDir := filepath.Join(outputDir, clusterID)
	if err := utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

	// Destroy cluster autoscaler.
	autoscalerBuilder := autoscaler.NewAutoscalerBuilder(req.ProjectName, req.Cluster, clusterDir)
	if err := autoscalerBuilder.DestroyClusterAutoscaler(); err != nil {
		logger.Err(err).Msgf("Error while destroying cluster autoscaler")
		return nil, fmt.Errorf("error while destroying cluster autoscaler for %s : %w", req.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Cluster autoscaler successfully destroyed")
	return &pb.DestroyClusterAutoscalerResponse{}, nil
}
