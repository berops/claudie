package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *GRPC) NextTask(ctx context.Context, _ *pb.NextTaskRequest) (*pb.NextTaskResponse, error) {
	nextTask := g.TaskQueue.Dequeue()
	if nextTask == nil {
		return nil, status.Errorf(codes.NotFound, "no tasks scheduled")
	}
	TasksInQueue.Dec()

	t := nextTask.(*EnqueuedTask)

	// before sending the task, we need to update its state to be marked as in progress.
	cfg, err := g.Store.GetConfig(ctx, t.Config)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q for which task %q was scheduled, aborting: %v", t.Config, t.Event.Id, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q exists, for which task %q was scheduled, aborting", t.Config, t.Event.Id)
	}

	grpcCfg, err := store.ConvertToGRPC(cfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert config %q from database representation to grpc: %v", cfg.Name, err)
	}

	// Perform validation (in case any changes have been made)
	cluster, exists := grpcCfg.Clusters[t.Cluster]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q for which task %q was scheduled, aborting", t.Cluster, t.Config, t.Event.Id)
	}

	// task that is always at the top of the queue is being worked on.
	if len(cluster.Events.Events) == 0 {
		return nil, status.Errorf(codes.NotFound, "failed to find task %q within cluster %q within config %q for which a task was scheduled, aborting", t.Event.Id, t.Cluster, t.Config)
	}
	if cluster.Events.Events[0].Id != t.Event.Id {
		return nil, status.Errorf(codes.NotFound, "task %q for cluster %q within config %q that was scheduled is not present in the state of the cluster", t.Event.Id, t.Cluster, t.Config)
	}

	cluster.State = &spec.Workflow{
		Status: spec.Workflow_IN_PROGRESS,
		Stage:  spec.Workflow_NONE,
	}

	cluster.Events.Ttl = TaskTTL

	if cluster.Current != nil {
		if err := transferExistingData(cluster, t.Event); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to re-use data from current state for desired state for config %q cluster %q: %v", grpcCfg.Name, t.Cluster, err)
		}
	}

	newConfig, err := store.ConvertFromGRPC(grpcCfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert grpc representation for config %q to database: %v", grpcCfg.Name, err)
	}

	if err := g.Store.UpdateConfig(ctx, newConfig); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "couldn't update config %q for which task %q was scheduled, dirty write", t.Config, t.Event.Id)
		}
		return nil, status.Errorf(codes.Internal, "failed to update task: %q for cluster: %q config: %q", t.Event.Id, t.Cluster, t.Config)
	}

	resp := &pb.NextTaskResponse{
		State:   cluster.State,
		Current: nil,
		Event:   t.Event,
		Ttl:     TaskTTL,
		Cluster: t.Cluster,
		Version: newConfig.Version,
		Config:  newConfig.Name,
	}

	if cluster.Current != nil {
		resp.Current = &spec.Clusters{
			K8S:           cluster.Current.K8S,
			LoadBalancers: cluster.Current.LoadBalancers,
		}
	}

	log.Info().Msgf("[%s] Task %q for cluster %q config %q has been picked up to work on", resp.Event.Event.String(), resp.Event.Id, resp.Cluster, resp.Config)

	return resp, nil
}

func transferExistingData(state *spec.ClusterState, te *spec.TaskEvent) error {
	switch te.Event {
	case spec.Event_UPDATE:
		if state.Events.Autoscaled {
			// autoscaler only deleted or adds node to the cluster
			// however since autoscaler can spawn multiple tasks
			// transfer only the relevant node data (no lb changes
			// or other changes are made by autoscaler) Autoscaler
			// also runs only if there are no other changes being
			// worked on and vice versa. We skip updating autoscaler config
			// as sets the desired nocepool count to the value from the current
			// state, which we don't want because the autoscaler event increases
			// and decreased the desired nodepool count which we want to build.
			for _, cnp := range state.Current.K8S.ClusterInfo.NodePools {
				if cnp.GetDynamicNodePool() == nil {
					continue
				}

				di := slices.IndexFunc(state.Desired.K8S.ClusterInfo.NodePools, func(pool *spec.NodePool) bool {
					return pool.Name == cnp.Name
				})
				if di < 0 {
					continue
				}

				dnp := state.Desired.K8S.ClusterInfo.NodePools[di]
				transferDynamicNpDataOnly(utils.GetClusterID(state.Desired.K8S.ClusterInfo), cnp, dnp, false)
			}
			return nil
		} else {
			if err := transferExistingK8sState(state.Current.K8S, te.Task.UpdateState.K8S); err != nil {
				return err
			}
			return transferExistingLBState(state.Current.LoadBalancers, te.Task.UpdateState.Lbs)
		}
	case spec.Event_DELETE, spec.Event_CREATE:
		return nil // do nothing.
	default:
		return fmt.Errorf("no such event recognized: %v", te.Event)
	}
}
