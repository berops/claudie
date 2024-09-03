package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *GRPC) TaskUpdate(ctx context.Context, req *pb.TaskUpdateRequest) (*pb.TaskUpdateResponse, error) {
	if req.Config == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if req.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}

	log.Debug().Msgf("Updating Config: %q Cluster: %q Version: %v Task: %q", req.Config, req.Cluster, req.Version, req.TaskId)

	cfg, err := g.Store.GetConfig(ctx, req.Config)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q: %v", req.Config, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q exists", req.Config)
	}
	if cfg.Version != req.Version {
		return nil, status.Errorf(codes.Aborted, "config %q with version %v was not found", req.Config, req.Version)
	}

	cluster, exists := cfg.Clusters[req.Cluster]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q", req.Cluster, req.Config)
	}

	// task that is always at the top of the queue is being worked on.
	if len(cluster.Events.TaskEvents) == 0 {
		return nil, status.Errorf(codes.NotFound, "failed to find task %q within cluster %q within config %q", req.TaskId, req.Cluster, req.Config)
	}
	if cluster.Events.TaskEvents[0].Id != req.TaskId {
		return nil, status.Errorf(codes.InvalidArgument, "cannot update task %q, as this task is not being currently worked on", req.TaskId)
	}

	cluster.State = store.ConvertFromGRPCWorkflow(req.State)

	switch {
	case cluster.State.Status == spec.Workflow_DONE.String():
		cluster.Events.TTL = 0
		cluster.Events.TaskEvents = cluster.Events.TaskEvents[1:]
		TasksFinishedOk.Inc()
	case cluster.State.Status == spec.Workflow_ERROR.String():
		cluster.Events.TTL = 0
		TasksFinishedErr.Inc()
	}

	if err := g.Store.UpdateConfig(ctx, cfg); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "couldn't update config %q with version %v, dirty write", req.Config, req.Version)
		}
		return nil, status.Errorf(codes.Internal, "failed to update task: %q for cluster: %q config: %q", req.TaskId, req.Cluster, req.Config)
	}

	return &pb.TaskUpdateResponse{Name: req.Config, Version: cfg.Version}, nil
}
