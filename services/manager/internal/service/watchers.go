package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
)

// TaskTTL is the minimum number of ticks (every ~10sec) within which a given task must be completed
// before being rescheduled again.
var TaskTTL = int32(envs.GetOrDefaultInt("BUILDER_TTL", 750 /* Default (750 * 10)/60/60 ~= 2hours */))

// Tick represents the interval at which each manifest state is checked.
const Tick = 10 * time.Second

func (g *GRPC) WatchForScheduledDocuments(ctx context.Context) error {
	cfgs, err := g.Store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Scheduled.String()}})
	if err != nil {
		return err
	}

	for _, scheduled := range cfgs {
		logger := loggerutils.WithProjectName(scheduled.Name)

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

			if state.Events.TTL > 0 {
				state.Events.TTL -= 1
				logger.Debug().Msgf("Decreasing TTL for task %q cluster %q", nextTask.Id, cluster)
				if err := g.Store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to decrement task TTL (%v) for cluster %q, dirty write", nextTask.Id, cluster)
						break
					}
					logger.Err(err).Msgf("Failed to decrement task TTL (%v) for cluster %q", nextTask.Id, cluster)
				}
				break
			}
		}

		if clustersDone == len(scheduled.Clusters) {
			var newManifestState manifest.State

			if anyError {
				newManifestState = manifest.Error
				logger.Info().Msgf("One of the clusters failed to build successfully, moving manifest to %q state", manifest.Error.String())
			} else {
				newManifestState = manifest.Done
				logger.Info().Msgf("All of the clusters build successfully, moving manifest to %q state", manifest.Done.String())
			}

			ok, err := manifest.ValidStateTransitionString(scheduled.Manifest.State, newManifestState)
			if err != nil || !ok {
				logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", scheduled.Manifest.State, newManifestState)
				continue
			}

			scheduled.Manifest.State = newManifestState.String()
			if err := g.Store.UpdateConfig(ctx, scheduled); err != nil {
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msgf("Scheduled Config couldn't be updated due to a Dirty Write")
					continue
				}
				logger.Err(err).Msgf("Failed to update scheduled config, skipping.")
				continue
			}
		}
	}

	return nil
}

func (g *GRPC) WatchForPendingDocuments(ctx context.Context) error {
	cfgs, err := g.Store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Pending.String()}})
	if err != nil {
		return err
	}

	for _, pending := range cfgs {
		logger := loggerutils.WithProjectName(pending.Name)
		logger.Info().Msgf("Processing Pending Config")

		if err := createDesiredState(pending); err != nil {
			logger.Err(err).Msgf("Failed to create desired state, skipping.")
			continue
		}

		result, err := scheduleTasks(pending)
		if err != nil {
			logger.Err(err).Msgf("Failed to create tasks, skipping.")
			continue
		}

		ok, err := manifest.ValidStateTransitionString(pending.Manifest.State, manifest.Scheduled)
		if err != nil || !ok {
			logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", pending.Manifest.State, manifest.Scheduled)
			continue
		}

		switch result {
		case NotReady:
			logger.Info().Msgf("manifest is not ready to be scheduled, retrying again later")
		case Reschedule:
			pending.Manifest.State = manifest.Scheduled.String()
			logger.Debug().Msgf("Scheduling for intermediate tasks after which the config will be rescheduled again")
		case FinalRetry, NoReschedule:
			logger.Debug().Msgf("Scheduling for tasks after which the config will not be rescheduled again")
			pending.Manifest.State = manifest.Scheduled.String()
			pending.Manifest.LastAppliedChecksum = pending.Manifest.Checksum
		}

		if err := g.Store.UpdateConfig(ctx, pending); err != nil {
			if errors.Is(err, store.ErrNotFoundOrDirty) {
				logger.Warn().Msgf("Pending Config couldn't be updated due to a Dirty Write, another retry will start shortly.")
				continue
			}
			logger.Err(err).Msgf("Failed to update pending config, skipping.")
			continue
		}

		switch result {
		case NotReady:
			// do nothing.
		case Reschedule, FinalRetry, NoReschedule:
			logger.Info().Msgf("Config has been successfully processed and moved to the %q state", manifest.Scheduled.String())
		}
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
		logger := loggerutils.WithProjectName(idle.Name)

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
					logger.Warn().Msgf("Idle Config couldn't be updated due to a Dirty Write, another retry will start shortly.")
					continue
				}
				logger.Err(err).Msgf("Failed to update idle config, skipping.")
				continue
			}

			logger.Info().Msgf("Config has been successfully processed and moved to the %q state", manifest.Pending.String())
			continue
		}

		if idle.Manifest.State == manifest.Done.String() {
			if idle.Manifest.Checksum == nil && idle.Manifest.LastAppliedChecksum == nil {
				if err := g.Store.DeleteConfig(ctx, idle.Name, idle.Version); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Warn().Msgf("Idle Config couldn't be deleted due to a Dirty Write, another retry will start shortly.")
						continue
					}
					logger.Err(err).Msgf("Failed to delete idle config, skipping.")
				}
				continue
			}

			clustersDeleted := false
			for cluster, state := range idle.Clusters {
				currentEmpty := len(state.Current.K8s) == 0 && len(state.Current.LoadBalancers) == 0
				desiredEmpty := len(state.Desired.K8s) == 0 && len(state.Desired.LoadBalancers) == 0

				if currentEmpty && desiredEmpty {
					logger.Debug().Msgf("Deleting cluster %q from database as infrastructure was destroyed", cluster)
					clustersDeleted = true
					delete(idle.Clusters, cluster)
				}
			}

			if clustersDeleted {
				if err := g.Store.UpdateConfig(ctx, idle); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Warn().Msgf("Idle Config couldn't be updated due to a Dirty Write, another retry will start shortly.")
						continue
					}
					logger.Err(err).Msgf("Failed to update idle config, skipping.")
				}
				continue
			}
		}

		cfg, err := store.ConvertToGRPC(idle)
		if err != nil {
			return fmt.Errorf("failed to convert database representation to grpc: %w", err)
		}

		updated, err := templatesUpdated(cfg)
		if err != nil {
			return fmt.Errorf("failed to check if templates for nodepools were updated: %w", err)
		}

		if updated {
			logger.Debug().Msgf("rescheduling idle config for rolling updates")
			// trigger reschedule.
			idle.Manifest.LastAppliedChecksum = append([]byte(nil), idle.Manifest.Checksum[1:]...)
			if err := g.Store.UpdateConfig(ctx, idle); err != nil {
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msgf("Idle config couldn't be rescheduled for rolling update due to a Dirty Write, another retry will start shortly.")
					continue
				}
				logger.Err(err).Msgf("Failed to reschedule Idle config for rolling update, skipping.")
			}
			continue
		}
	}

	return nil
}
