package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *GRPC) TaskUpdate(ctx context.Context, req *pb.TaskUpdateRequest) (*pb.TaskUpdateResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if req.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}
	if req.TaskId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing task id")
	}

	switch action := req.Action.(type) {
	case *pb.TaskUpdateRequest_State:
		if action.State.Status == spec.Workflow_DONE || action.State.Status == spec.Workflow_ERROR {
			return nil, status.Errorf(codes.InvalidArgument, "to complete a task, use the TaskComplete RPC")
		}
		log.Debug().Msgf("Updating Config: %q Cluster: %q Version: %v Task: %q with status: %q", req.Name, req.Cluster, req.Version, req.TaskId, action.State.Status.String())
	case *pb.TaskUpdateRequest_Refresh_:
		log.Debug().Msgf("Refreshing Lease for Task: %q for Cluster: %q Version: %v for Config: %q", req.TaskId, req.Cluster, req.Version, req.Name)
		// do nothing.
	default:
		return nil, status.Errorf(codes.InvalidArgument, "undefined action %T", action)
	}

	cfg, err := g.Store.GetConfig(ctx, req.Name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q: %v", req.Name, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q exists", req.Name)
	}
	if cfg.Version != req.Version {
		return nil, status.Errorf(codes.Aborted, "config %q with version %v was not found", req.Name, req.Version)
	}

	cluster, exists := cfg.Clusters[req.Cluster]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q", req.Cluster, req.Name)
	}

	// task that is always at the top of the queue is being worked on.
	if len(cluster.Events.TaskEvents) == 0 {
		log.Debug().Msgf("Failed updating Config: %q Cluster: %q Version: %v Task: %q. No tasks in queue", req.Name, req.Cluster, req.Version, req.TaskId)
		return nil, status.Errorf(codes.NotFound, "failed to find task %q within cluster %q within config %q", req.TaskId, req.Cluster, req.Name)
	}
	if cluster.Events.TaskEvents[0].Id != req.TaskId {
		log.Debug().Msgf("Failed updating Config: %q Cluster: %q Version: %v Task: %q, does, not match top level task", req.Name, req.Cluster, req.Version, req.TaskId)
		return nil, status.Errorf(codes.NotFound, "cannot update task %q, as this task is not being currently worked on", req.TaskId)
	}

	// Reset the Timer for the Lease in all actions.
	switch action := req.Action.(type) {
	case *pb.TaskUpdateRequest_State:
		cluster.State = store.ConvertFromGRPCWorkflow(action.State)

		cluster.Events.Lease.RemainingMissedRefreshCount = AllowedMissedLeaseRefresh
		cluster.Events.Lease.RemainingTicksForRefresh = TaskLeaseTime
	case *pb.TaskUpdateRequest_Refresh_:
		cluster.Events.Lease.RemainingMissedRefreshCount = AllowedMissedLeaseRefresh
		cluster.Events.Lease.RemainingTicksForRefresh = TaskLeaseTime
	}

	if err := g.Store.UpdateConfig(ctx, cfg); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			log.Debug().Msgf("Failed updating (dirty write) Config: %q Cluster: %q Version: %v Task: %q", req.Name, req.Cluster, req.Version, req.TaskId)
			return nil, status.Errorf(codes.Aborted, "couldn't update config %q with version %v, dirty write", req.Name, req.Version)
		}

		log.Debug().Msgf("Failed updating Config: %q Cluster: %q Version: %v Task: %q", req.Name, req.Cluster, req.Version, req.TaskId)
		return nil, status.Errorf(codes.Internal, "failed to update task: %q for cluster: %q config: %q", req.TaskId, req.Cluster, req.Name)
	}

	log.Debug().Msgf("Updated Config: %q Cluster: %q Version: %v Task: %q with status: %s", req.Name, req.Cluster, req.Version, req.TaskId, cluster.State.Status)

	return &pb.TaskUpdateResponse{Name: req.Name, Version: cfg.Version}, nil
}
