package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (g *GRPC) NextTask(ctx context.Context, _ *pb.NextTaskRequest) (*pb.NextTaskResponse, error) {
	cfgs, err := g.Store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Scheduled.String()}})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve next task %v", err)
	}

	cfg, clusterName := nextTask(cfgs)
	if cfg == nil {
		return nil, status.Errorf(codes.NotFound, "no tasks schedulable")
	}

	grpcCfg, err := store.ConvertToGRPC(cfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert config %q from database representation to grpc: %v", cfg.Name, err)
	}

	cluster := grpcCfg.Clusters[clusterName]
	events := cluster.Events.Events

	outgoingTask := proto.Clone(events[0]).(*spec.TaskEvent)

	cluster.State = &spec.Workflow{
		Status: spec.Workflow_IN_PROGRESS,
		Stage:  spec.Workflow_NONE,
	}

	cluster.Events.Ttl = TaskTTL

	if cluster.Current != nil {
		log.Debug().Str("cluster", cluster.Current.K8S.ClusterInfo.Id()).Msgf("transferring existing state into %s task %q", outgoingTask.Event.String(), outgoingTask.Id)
		if err := transferExistingData(cluster, outgoingTask); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to re-use data from current state for desired state for config %q cluster %q: %v", grpcCfg.Name, clusterName, err)
		}
	}

	newConfig, err := store.ConvertFromGRPC(grpcCfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert grpc representation for config %q to database: %v", grpcCfg.Name, err)
	}

	if err := g.Store.UpdateConfig(ctx, newConfig); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "couldn't update config %q for which task %q was scheduled, dirty write", cfg.Name, outgoingTask.Id)
		}
		return nil, status.Errorf(codes.Internal, "failed to update task: %q for cluster: %q config: %q", outgoingTask.Id, clusterName, cfg.Name)
	}

	resp := &pb.NextTaskResponse{
		State:   cluster.State,
		Current: nil,
		Event:   outgoingTask,
		Ttl:     TaskTTL,
		Cluster: clusterName,
		Version: newConfig.Version,
		Name:    newConfig.Name,
	}

	if cluster.Current != nil {
		resp.Current = &spec.Clusters{
			K8S:           cluster.Current.K8S,
			LoadBalancers: cluster.Current.LoadBalancers,
		}
	}

	log.Info().Msgf("[%s] Task %v (%v) for cluster %q config %q has been picked up to work on", resp.Event.Event.String(), resp.Event.Description, resp.Event.Id, resp.Cluster, resp.Name)

	TasksScheduled.Inc()

	return resp, nil
}

func nextTask(cfgs []*store.Config) (*store.Config, string) {
	for _, cfg := range cfgs {
		for c, s := range cfg.Clusters {
			if s.Events.TTL == 0 && len(s.Events.TaskEvents) > 0 && s.State.Status != spec.Workflow_ERROR.String() {
				return cfg, c
			}
		}
	}
	return nil, ""
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
				dnp := nodepools.FindByName(cnp.Name, state.Desired.K8S.ClusterInfo.NodePools)
				if dnp == nil {
					continue
				}
				transferDynamicNp(state.Desired.K8S.ClusterInfo.Id(), cnp, dnp, false)
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
