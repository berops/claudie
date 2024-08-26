package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/syncqueue"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
)

// TaskTTL is the minimum number of ticks (every ~10sec) within which a given task must be completed
// before being rescheduled again.
const TaskTTL = 450 // ~1.1 hour

// TODO: fixup log messages so that they don't contain the Config name twice.

type EnqueuedTask struct {
	Config  string
	Cluster string
	Event   *spec.TaskEvent
	TTL     int32
	Version uint64
}

func (t *EnqueuedTask) ID() string { return t.Event.Id }

func (g *GRPC) WatchForScheduledDocuments(ctx context.Context) error {
	cfgs, err := g.Store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Scheduled.String()}})
	if err != nil {
		return err
	}

	for _, scheduled := range cfgs {
		logger := utils.CreateLoggerWithProjectName(scheduled.Name)

		// clusterDone counts the number of cluster for which no progress is currently being made.
		var clustersDone int
		var anyError bool

		for cluster, state := range scheduled.Clusters {
			if len(state.Events.TaskEvents) == 0 {
				clustersDone++
				continue
			}

			if state.State.Status == spec.Workflow_ERROR.String() {
				// We count as done as no further work will be done with
				// this cluster even though the state is in error.
				clustersDone++
				anyError = true
				continue
			}

			nextTask := state.Events.TaskEvents[0]
			if g.TaskQueue.Contains(&EnqueuedTask{Event: &spec.TaskEvent{Id: nextTask.Id}}) {
				continue
			}

			if state.Events.TTL > 0 {
				state.Events.TTL -= 1
				logger.Debug().Msgf("Decreasing TTL for task %q cluster %q config %q", nextTask.Id, cluster, scheduled.Name)
				if err := g.Store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to decrement task TTL for cluster %q config %q, dirty write", cluster, scheduled.Name)
						continue
					}
					logger.Err(err).Msgf("Failed to decrement task TTL for cluster %q config %q", cluster, scheduled.Name)
				}
				continue
			}

			logger.Debug().Msgf("Scheduling next task with ID: %v for cluster %q config %q", nextTask.Id, cluster, scheduled.Name)
			if err := addTaskToQueue(g.TaskQueue, scheduled.Name, cluster, scheduled.Version, state); err != nil {
				logger.Err(err).Msgf("Failed to add task %v for cluster %q config %q to the task queue", nextTask.Id, cluster, scheduled.Name)
			}
			logger.Info().Msgf("Task %v for cluster %v config %v scheduled", nextTask.Id, cluster, scheduled.Name)
		}

		if clustersDone == len(scheduled.Clusters) {
			var newManifestState manifest.State

			if anyError {
				newManifestState = manifest.Error
				logger.Info().Msgf("One of the clusters failed to build sucesfully, moving manifest to %q state", manifest.Error.String())
			} else {
				newManifestState = manifest.Done
				logger.Info().Msgf("All of the clusters build sucesfully, moving manifest to %q state", manifest.Done.String())
			}

			ok, err := manifest.ValidStateTransitionString(scheduled.Manifest.State, newManifestState)
			if err != nil || !ok {
				logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", scheduled.Manifest.State, manifest.Done)
				continue
			}

			scheduled.Manifest.State = newManifestState.String()
			if err := g.Store.UpdateConfig(ctx, scheduled); err != nil {
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msgf("Scheduled Config %q couldn't be updated due to a Dirty Write", scheduled.Name)
					continue
				}
				logger.Err(err).Msgf("Failed to update scheduled config %q, skipping.", scheduled.Name)
				continue
			}
		}
	}

	return nil
}

func addTaskToQueue(queue *syncqueue.Queue, config, cluster string, version uint64, state *store.ClusterState) error {
	te, err := store.ConvertToGRPCTaskEvent(state.Events.TaskEvents[0])
	if err != nil {
		return fmt.Errorf("failed to convert database representation GRPC: %w", err)
	}

	w := &EnqueuedTask{
		Config:  config,
		Cluster: cluster,
		Event:   te,
		TTL:     state.Events.TTL,
		Version: version,
	}

	queue.Enqueue(w)
	return nil
}

func (g *GRPC) WatchForPendingDocuments(ctx context.Context) error {
	cfgs, err := g.Store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Pending.String()}})
	if err != nil {
		return err
	}

	for _, pending := range cfgs {
		logger := utils.CreateLoggerWithProjectName(pending.Name)
		logger.Info().Msgf("Processing Pending Config")

		if err := createDesiredState(pending); err != nil {
			logger.Err(err).Msgf("Failed to create desired state, skipping.")
			continue
		}

		if err := scheduleTasks(pending); err != nil {
			logger.Err(err).Msgf("Failed to create tasks, skipping.")
			continue
		}

		ok, err := manifest.ValidStateTransitionString(pending.Manifest.State, manifest.Scheduled)
		if err != nil || !ok {
			logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", pending.Manifest.State, manifest.Scheduled)
			continue
		}

		pending.Manifest.State = manifest.Scheduled.String()
		pending.Manifest.LastAppliedChecksum = pending.Manifest.Checksum

		if err := g.Store.UpdateConfig(ctx, pending); err != nil {
			if errors.Is(err, store.ErrNotFoundOrDirty) {
				logger.Warn().Msgf("Pending Config %q couldn't be updated due to a Dirty Write, another retry will start shortly.", pending.Name)
				continue
			}
			logger.Err(err).Msgf("Failed to update pending config %q, skipping.", pending.Name)
			continue
		}

		logger.Info().Msgf("Config has been sucessfully processed and has been moved to the %q state", manifest.Scheduled.String())
	}

	return nil
}

func (g *GRPC) WatchForDoneOrErrorDocuments(ctx context.Context) error {
	cfgs, err := g.Store.ListConfigs(ctx, &store.ListFilter{
		ManifestState: []string{
			manifest.Done.String(),
			manifest.Error.String(),
		},
	})
	if err != nil {
		return err
	}

	for _, idle := range cfgs {
		logger := utils.CreateLoggerWithProjectName(idle.Name)

		if !bytes.Equal(idle.Manifest.LastAppliedChecksum, idle.Manifest.Checksum) {
			logger.Info().Msgf("Moving to %q as changes have been made to the manifest since the last build", manifest.Pending.String())

			ok, err := manifest.ValidStateTransitionString(idle.Manifest.State, manifest.Pending)
			if err != nil || !ok {
				logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", idle.Manifest.State, manifest.Pending)
				continue
			}

			idle.Manifest.State = manifest.Pending.String()

			if err := g.Store.UpdateConfig(ctx, idle); err != nil {
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msgf("Idle Config %q couldn't be updated due to a Dirty Write, another retry will start shortly.", idle.Name)
					continue
				}
				logger.Err(err).Msgf("Failed to update idle config %q, skipping.", idle.Name)
				continue
			}

			logger.Info().Msgf("Config has been sucessfully processed and has been moved to the %q state", manifest.Pending.String())
			continue
		}

		if idle.Manifest.State == manifest.Done.String() {
			if idle.Manifest.Checksum == nil && idle.Manifest.LastAppliedChecksum == nil {
				if err := g.Store.DeleteConfig(ctx, idle.Name, idle.Version); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Warn().Msgf("Idle Config %q couldn't be deleted due to a Dirty Write, another retry will start shortly.", idle.Name)
						continue
					}
					logger.Err(err).Msgf("Failed to delete idle config %q, skipping.", idle.Name)
				}
				continue
			}

			clustersDeleted := false
			for cluster, state := range idle.Clusters {
				currentEmpty := len(state.Current.K8s) == 0 && len(state.Current.LoadBalancers) == 0
				desiredEmpty := len(state.Desired.K8s) == 0 && len(state.Desired.LoadBalancers) == 0

				if currentEmpty && desiredEmpty {
					clustersDeleted = true
					delete(idle.Clusters, cluster)
				}
			}

			if clustersDeleted {
				if err := g.Store.UpdateConfig(ctx, idle); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Warn().Msgf("Idle Config %q couldn't be updated due to a Dirty Write, another retry will start shortly.", idle.Name)
						continue
					}
					logger.Err(err).Msgf("Failed to update idle config %q, skipping.", idle.Name)
				}
				continue
			}
		}
	}

	return nil
}
