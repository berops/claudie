package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) MarkNodeForDeletion(ctx context.Context, request *pb.MarkNodeForDeletionRequest) (*pb.MarkNodeForDeletionResponse, error) {
	if request.Config == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if request.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}
	if request.Nodepool == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing nodepool name")
	}
	if request.Node == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing node name")
	}

	log.
		Debug().
		Msgf("Marking node %q within nodepool %q within config %q for deletion",
			request.Node,
			request.Nodepool,
			request.Config,
		)

	cfg, err := s.store.GetConfig(ctx, request.Config)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(
				codes.Internal,
				"failed to check existence of config %q: %v", request.Config, err,
			)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q exists", request.Config)
	}

	cs := cfg.Clusters[request.Cluster]
	if !cs.Exists() {
		return nil, status.Errorf(codes.NotFound, "no cluster %q found within config %q", request.Cluster, request.Config)
	}

	state, err := store.ConvertToGRPCClusterState(cs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert cluster state database representation to grpc: %v", err)
	}

	var nodepool *spec.NodePool
	if request.Loadbalancer == nil {
		nodepool = nodepools.FindByName(request.Nodepool, state.Current.K8S.ClusterInfo.NodePools)
	} else {
		idx := clusters.IndexLoadbalancerById(*request.Loadbalancer, state.Current.LoadBalancers.Clusters)
		if idx < 0 {
			return nil, status.Errorf(
				codes.Internal,
				"no loadbalancer %q attached to cluster %q, config %q", *request.Loadbalancer, request.Cluster, request.Config,
			)
		}
		nodepool = nodepools.FindByName(request.Nodepool, state.Current.LoadBalancers.Clusters[idx].ClusterInfo.NodePools)
	}

	if nodepool == nil {
		return nil, status.Errorf(
			codes.NotFound,
			"specified nodepool %q not found within config %q", request.Nodepool, request.Config,
		)
	}

	var node *spec.Node
	for _, n := range nodepool.Nodes {
		if n.Name == request.Node {
			node = n
			break
		}
	}

	if node == nil {
		return nil, status.Errorf(
			codes.NotFound,
			"specified node %q not found within nodepool %q in config %q", request.Node, request.Nodepool, request.Config,
		)
	}

	// Update node.
	// For static nodepools the target size is simply the
	// number of nodes and does not change here.
	currentTargetSize := int64(len(nodepool.Nodes))
	if dyn := nodepool.GetDynamicNodePool(); dyn != nil {
		// For dynamic nodepools its the count specified in the
		// InputManifest and does not change here.
		currentTargetSize = int64(dyn.Count)

		if dyn.AutoscalerConfig != nil {
			if request.ShouldDecrementDesiredCapacity != nil && *request.ShouldDecrementDesiredCapacity {
				newTargetSize := dyn.AutoscalerConfig.TargetSize - 1
				if newTargetSize < dyn.AutoscalerConfig.Min {
					return nil, status.Errorf(
						codes.InvalidArgument,
						"decreasing the desired capacity for the autoscaled nodepool %q would result"+
							" in a lower value than the allowed Minimum value", request.Nodepool,
					)
				}

				dyn.AutoscalerConfig.TargetSize = newTargetSize
			}

			// For autoscaled nodepools its the TargetSize.
			currentTargetSize = int64(dyn.AutoscalerConfig.TargetSize)
		}
	}
	node.Status = spec.NodeStatus_MarkedForDeletion

	// Persist changes.
	db, err := store.ConvertFromGRPCClusterState(state)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert grpc representation to database: %v", err)
	}

	cfg.Clusters[request.Cluster] = db
	if err := s.store.UpdateConfig(ctx, cfg); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(
				codes.Aborted,
				"couldn't update config %q with version %v, dirty write", request.Config, cfg.Version,
			)
		}
		return nil, status.Errorf(
			codes.Internal,
			"failed to update current state for cluster: %q config: %q", request.Cluster, request.Config,
		)
	}

	return &pb.MarkNodeForDeletionResponse{TargetSize: currentTargetSize}, nil
}
