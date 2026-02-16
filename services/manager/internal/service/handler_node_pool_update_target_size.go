package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) NodePoolUpdateTargetSize(ctx context.Context, request *pb.NodePoolUpdateTargetSizeRequest) (*pb.NodePoolUpdateTargetSizeResponse, error) {
	if request.Config == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if request.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}
	if request.Nodepool == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing nodepool name")
	}

	log.
		Debug().
		Msgf("Updating nodepool %q within config %q with target size: %v",
			request.Nodepool,
			request.Config,
			request.TargetSize,
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

	if cs.InFlight != nil {
		return nil, status.Errorf(codes.Internal, "cluster has on going changes, try again later")
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

	// For static nodepools the target size is simly
	// the number of nodes and does not change here.
	currentTargetSize := int32(len(nodepool.Nodes))
	if dyn := nodepool.GetDynamicNodePool(); dyn != nil {
		// For dynamic nodepools its the count specified
		// in the InputManifest and does not change here.
		currentTargetSize = dyn.Count

		if dyn.AutoscalerConfig != nil {
			// For autoscaled nodepools, it is allowed to modify the
			// target size as it is externally managed, i.e. not dictated
			// by the InputManifests desired state.

			if request.TargetSize < dyn.AutoscalerConfig.Min {
				return nil, status.Errorf(
					codes.InvalidArgument,
					"new target size %v is lower than the [%v, %v] range of the autoscaler",
					request.TargetSize,
					dyn.AutoscalerConfig.Min,
					dyn.AutoscalerConfig.Max,
				)
			}

			if request.TargetSize > dyn.AutoscalerConfig.Max {
				return nil, status.Errorf(
					codes.InvalidArgument,
					"new target size %v is higher than the [%v, %v] range of the autoscaler",
					request.TargetSize,
					dyn.AutoscalerConfig.Min,
					dyn.AutoscalerConfig.Max,
				)
			}

			dyn.AutoscalerConfig.TargetSize = request.TargetSize

			currentTargetSize = dyn.AutoscalerConfig.TargetSize
		}
	}

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

	return &pb.NodePoolUpdateTargetSizeResponse{TargetSize: currentTargetSize}, nil
}
