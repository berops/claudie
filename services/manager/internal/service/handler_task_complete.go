package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *GRPC) TaskComplete(ctx context.Context, req *pb.TaskCompleteRequest) (*pb.TaskCompleteResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if req.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}
	if req.TaskId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing task id")
	}
	if req.Workflow.Status != spec.Workflow_DONE && req.Workflow.Status != spec.Workflow_ERROR {
		return nil, status.Errorf(codes.InvalidArgument, "can only complete a task by marking it as \"DONE\" or \"ERROR\"")
	}

	log.Debug().Msgf("Completing task: %q from Config: %q Cluster: %q Version: %v, with status: %q", req.TaskId, req.Name, req.Cluster, req.Version, req.Workflow.Status.String())

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

	converted, err := store.ConvertToGRPC(cfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", req.Name, err)
	}

	cluster, exists := converted.Clusters[req.Cluster]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q", req.Cluster, req.Name)
	}

	// task that is always at the top of the queue is being worked on.
	if len(cluster.Events.Events) == 0 {
		log.Debug().Msgf("Failed completing task %q from Config: %q Cluster: %q Version: %v. No tasks in queue", req.TaskId, req.Name, req.Cluster, req.Version)
		return nil, status.Errorf(codes.NotFound, "failed to find task %q within cluster %q within config %q", req.TaskId, req.Cluster, req.Name)
	}
	if cluster.Events.Events[0].Id != req.TaskId {
		log.Debug().Msgf("Failed completing task %q from Config: %q Cluster: %q Version: %v, does not match top level task", req.TaskId, req.Name, req.Cluster, req.Version)
		return nil, status.Errorf(codes.NotFound, "cannot update task %q, as this task is not being currently worked on", req.TaskId)
	}

	cluster.State = req.Workflow
	switch {
	case req.Workflow.Status == spec.Workflow_DONE:
		cluster.Events.Ttl = 0
		cluster.Events.Events = cluster.Events.Events[1:]
		TasksFinishedOk.Inc()
		log.Debug().Msgf("Completing task %q from Config: %q Cluster: %q Version: %v, Finished successfully", req.TaskId, req.Name, req.Cluster, req.Version)
	case req.Workflow.Status == spec.Workflow_ERROR:
		cluster.Events.Ttl = 0
		TasksFinishedErr.Inc()
		log.Debug().Msgf("Completing task %q from Config: %q Cluster: %q Version: %v, Errored", req.TaskId, req.Name, req.Cluster, req.Version)
	}

	cluster.Current = req.State

	if cluster.Current != nil && cluster.Desired != nil { // on update.
		log.Debug().Str("cluster", utils.GetClusterID(cluster.Current.K8S.ClusterInfo)).Msgf("transferring state from newly supplied current state into desired state")

		if err := transferExistingK8sState(cluster.Current.K8S, cluster.Desired.K8S); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to trasnsfer updated current state to desired state for task %q cluster %q config %q: %v", req.TaskId, req.Cluster, req.Name, err)
		}

		if err := transferExistingLBState(cluster.Current.LoadBalancers, cluster.Desired.LoadBalancers); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to trasnsfer updated current state to desired state for task %q cluster %q config %q: %v", req.TaskId, req.Cluster, req.Name, err)
		}
	}

	cfg, err = store.ConvertFromGRPC(converted)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert config %q from grpc representation to database representation: %v", req.Name, err)
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

	return &pb.TaskCompleteResponse{Name: cfg.Name, Version: cfg.Version}, nil
}
