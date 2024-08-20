package service

import (
	"context"
	"errors"
	"time"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (g *GRPC) NextTask(ctx context.Context, _ *pb.NextTaskRequest) (*pb.NextTaskResponse, error) {
	nextTask := g.TaskQueue.Dequeue()
	if nextTask == nil {
		return nil, status.Errorf(codes.NotFound, "no tasks scheduled")
	}

	t := nextTask.(*EnqueuedTask)

	// before sending the task, we need to update its state to be marked as in progress.
	cfg, err := g.Store.GetConfig(ctx, t.Config)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q for which task %q was scheduled, aborting: %v", t.Config, t.Event.Id, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q exists, for which task %q was scheduled, aborting", t.Config, t.Event.Id)
	}

	// Perform validation (in case any changes have been made)
	cluster, exists := cfg.Clusters[t.Cluster]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q for which task %q was scheduled, aborting", t.Cluster, t.Config, t.Event.Id)
	}

	// task that is always at the top of the queue is being worked on.
	if len(cluster.Events.TaskEvents) == 0 {
		return nil, status.Errorf(codes.NotFound, "failed to find task %q within cluster %q within config %q for which a task was scheduled, aborting", t.Event.Id, t.Cluster, t.Config)
	}
	if cluster.Events.TaskEvents[0].Id != t.Event.Id {
		return nil, status.Errorf(codes.NotFound, "task %q for cluster %q within config %q that was scheduled is not present in the state of the cluster", t.Event.Id, t.Cluster, t.Config)
	}

	var k8s spec.K8Scluster
	if err := proto.Unmarshal(cluster.Current.K8s, &k8s); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal database representation of k8s cluster to GRPC: %v", err)
	}

	var lbs spec.LoadBalancers
	if err := proto.Unmarshal(cluster.Current.LoadBalancers, &lbs); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal database representation of lb clusters GRPC: %v", err)
	}

	cluster.State = store.Workflow{
		Status:    spec.Workflow_IN_PROGRESS.String(),
		Stage:     spec.Workflow_NONE.String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	cluster.Events.TTL = TaskTTL
	if err := g.Store.UpdateConfig(ctx, cfg); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "couldn't update config %q for which task %q was scheduled, dirty write", t.Config, t.Event.Id)
		}
		return nil, status.Errorf(codes.Internal, "failed to update task: %q for cluster: %q config: %q", t.Event.Id, t.Cluster, t.Config)
	}

	resp := &pb.NextTaskResponse{
		State: store.ConvertToGRPCWorkflow(cluster.State),
		Current: &spec.Clusters{
			K8S:           &k8s,
			LoadBalancers: &lbs,
		},
		Event:   t.Event,
		Ttl:     t.TTL,
		Cluster: t.Cluster,
		Version: cfg.Version,
		Config:  cfg.Name,
	}

	log.Info().Msgf("Task %q for cluster %q config %q has been picked up to work on", resp.Event.Id, resp.Cluster, resp.Config)

	return resp, nil
}
