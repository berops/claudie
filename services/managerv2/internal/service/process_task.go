package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	Work struct {
		InputManifest string
		Cluster       string
		TaskID        string
		Stage         store.StageKind
		Result        *spec.TaskResult
	}

	Stores struct {
		store store.Store
	}
)

func ProcessTask(ctx context.Context, stores Stores, work Work) (acknowledge bool) {
	logger, ok := loggerutils.Value(ctx)
	if !ok {
		logger = log.With().Logger()
		logger.Warn().Msg("No logger attached, using default")
	}

	im, err := stores.store.GetConfig(ctx, work.InputManifest)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			// don't acknowledge the message, do a retry as it could
			// be a simple network problem or context cancelation.
			logger.Err(err).Msg("Failed to get input manifest")
			acknowledge = false
			return
		}
		acknowledge = true
		return
	}

	cluster, ok := im.Clusters[work.Cluster]
	if !ok {
		logger.Warn().Msg("Cluster within InputManifest not found, ignoring.")
		acknowledge = true
		return
	}

	if cluster.InFlight == nil || (cluster.InFlight.Id != work.TaskID) {
		logger.Warn().Msg("Recevied task for cluster does not match, ignoring.")
		acknowledge = true
		return
	}

	stage := cluster.InFlight.Pipeline[cluster.InFlight.CurrentStage]
	if stage.Kind != work.Stage {
		logger.
			Warn().
			Msgf(
				"Received task for cluster from stage %s, expected %s, ignoring",
				work.Stage,
				stage,
			)
		acknowledge = true
		return
	}

	if err := work.Result.Error; err != nil {
		acknowledge = processTaskWithError(ctx, logger, im, stores, work)
		return
	}

	if err := propagateResult(logger, cluster, work.Result); err != nil {
		// Parsing of the database representation shouldn't fail, there has to
		// be some kind of malformed or corrupted data, thus don't acknowledge the
		// received message, halting the workflow.
		acknowledge = false
		return
	}

	if err := advanceToNextStage(logger, cluster); err != nil {
		// Parsing of the database representation shouldn't fail, there has to
		// be some kind of malformed or corrupted data, thus don't acknowledge the
		// received message, halting the workflow.
		acknowledge = false
		return
	}

	if err := stores.store.UpdateConfig(ctx, im); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			logger.Debug().Msg("Failed to update InputManifest, dirty write")
		} else {
			logger.Err(err).Msg("Failed to update InputManifest")
		}
		acknowledge = false
		return
	}

	logger.
		Info().
		Msg("Successfully updated current state of the cluster. Moved the Task to the next stage")

	acknowledge = true
	return
}

func processTaskWithError(
	ctx context.Context,
	logger zerolog.Logger,
	im *store.Config,
	stores Stores,
	work Work,
) (acknowledge bool) {
	logger.
		Error().
		Msgf("Task resulted in an error during the current stage: %s", work.Result.Error.Description)

	var (
		cluster = im.Clusters[work.Cluster]
		stage   = cluster.InFlight.Pipeline[cluster.InFlight.CurrentStage]

		isErrorPartial = work.Result.Error.Kind == spec.TaskResult_Error_PARTIAL
		isStageWarn    = stage.Description.ErrorLevel == spec.ErrorLevel_ERROR_WARN.String()
	)

	cluster.State.Status = spec.WorkflowV2_ERROR.String()
	cluster.State.Description = work.Result.Error.Description

	if isErrorPartial {
		if err := propagateResult(logger, cluster, work.Result); err != nil {
			// Since the function works with successfully loaded database data and
			// also the successfully received work result data, there has to be some kind
			// of malformed or corrupted data, thus we don't acknowledge the received message, halting
			// the workflow.
			acknowledge = false
			return
		}
	}

	if isStageWarn {
		if err := advanceToNextStage(logger, cluster); err != nil {
			// Parsing of the database representation shouldn't fail, there has to
			// be some kind of malformed or corrupted data, thus don't acknowledge the
			// received message, halting the workflow.
			acknowledge = false
			return
		}
	}

	if err := stores.store.UpdateConfig(ctx, im); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			logger.Debug().Msg("Failed to update InputManifest, dirty write")
		} else {
			logger.Err(err).Msg("Failed to update InputManifest")
		}
		acknowledge = false
		return
	}

	logger.Info().Msg("Successfully processed task that ended up in error.")
	acknowledge = true
	return
}

func advanceToNextStage(logger zerolog.Logger, state *store.ClusterState) error {
	state.InFlight.CurrentStage += 1

	if int(state.InFlight.CurrentStage) < len(state.InFlight.Pipeline) {
		logger.
			Info().
			Msgf("Advancing task to the next stage %s", state.InFlight.Pipeline[state.InFlight.CurrentStage].Kind)

		state.State.Status = spec.WorkflowV2_WAIT_FOR_PICKUP.String()
		state.State.Description = state.InFlight.Pipeline[state.InFlight.CurrentStage].Description.About
		return nil
	}

	if err := moveInFlightStateToCurrentState(state); err != nil {
		return err
	}

	state.InFlight = nil
	state.State.Status = spec.WorkflowV2_DONE.String()
	state.State.Description = ""

	logger.Info().Msg("Task successfully finished moving to DONE")
	return nil
}

func propagateResult(logger zerolog.Logger, cluster *store.ClusterState, result *spec.TaskResult) error {
	inFlight, err := store.ConvertToGRPCTaskEvent(cluster.InFlight)
	if err != nil {
		logger.Err(err).Msg("Failed to unmarshal database representation")
		return err
	}

	switch result := result.Result.(type) {
	case *spec.TaskResult_Update:
		logger.Debug().Msg("Received [Update] as a result for the task")
		if err := inFlight.Task.ConsumeUpdateResult(result); err != nil {
			logger.
				Err(err).
				Msg("Unexpected mismatch in the name of the clusters from the received messagge and current state in the database, ignoring")
		}
	case *spec.TaskResult_Clear:
		logger.Debug().Msg("Received [Clear] as a result for the task")
		inFlight.Task.ConsumeClearResult(result)
	case *spec.TaskResult_None_:
		logger.Debug().Msg("Received [None] as a result for the task, no work to be done.")
	default:
		logger.Warn().Msgf("Received message with unknown result type %T, ignoring", result)
	}

	state, err := inFlight.Task.MutableClusters()
	if err != nil {
		logger.Err(err).Msg("Failed to acquire mutable state from scheduled task")
		return err
	}

	// The [store.ClusterState.InFlight.Task] can have multiple pipeline stages, thus
	// transfer the inFlight state, back to the scheduled task.
	switch task := inFlight.Task.Do.(type) {
	case *spec.TaskV2_Delete, *spec.TaskV2_Create:
		// Create and Delete task directly store the [spec.Clusters] state within
		// the scheduled tasks, therefore there is nothing left to be propagated
		// back as everything was handled by the propagation at the start of this
		// function.
		//
		// do nothing.
	case *spec.TaskV2_Update:
		// Contrary to the other scheduled tasks, Update task do not only have
		// the [spec.Clusters] state stored but also additional state that
		// needs to be updated, for example when Adding/Deleting loadbalancers
		// replacing Dns etc...
		logger.Debug().Msg("Propagating updated state back to [Update] task")

		// Replace the scheduled 'delta' with the updated state, if any, from the
		// updated [spec.Clusters], propagated from the task result.
		switch delta := task.Update.Delta.(type) {
		case *spec.UpdateV2_AddLoadBalancer_:
			u := delta.AddLoadBalancer
			id := u.LoadBalancer.ClusterInfo.Id()
			if i := clusters.IndexLoadbalancerByIdV2(id, state.LoadBalancers.Clusters); i >= 0 {
				u.LoadBalancer = state.LoadBalancers.Clusters[i]
			}
		case *spec.UpdateV2_ApiEndpoint_:
			// nothing to update.
		case *spec.UpdateV2_ClusterApiPort:
			// nothing to update.
		case *spec.UpdateV2_DeleteLoadBalancer_:
			// Deletion works with current state that is
			// not partially modified in any way.
			//
			// nothing to update.
		case *spec.UpdateV2_ReconcileLoadBalancer_:
			u := delta.ReconcileLoadBalancer
			id := u.LoadBalancer.ClusterInfo.Id()
			if i := clusters.IndexLoadbalancerByIdV2(id, state.LoadBalancers.Clusters); i >= 0 {
				u.LoadBalancer = state.LoadBalancers.Clusters[i]
			}
		case *spec.UpdateV2_ReplaceDns_:
			id := delta.ReplaceDns.LoadBalancerId
			if i := clusters.IndexLoadbalancerByIdV2(id, state.LoadBalancers.Clusters); i >= 0 {
				updated := state.LoadBalancers.Clusters[i]
				delta.ReplaceDns.Dns = updated.Dns
			}
		default:
			logger.Warn().Msgf("Unknown update delta %T, ignoring propagating updated state", delta)
		}

	default:
		logger.Warn().Msgf("Unsupported InFlight Task %T, ignoring transferring InFlight state", task)
	}

	cluster.InFlight, err = store.ConvertFromGRPCTaskEvent(inFlight)
	if err != nil {
		logger.Err(err).Msg("Failed to marshal grpc representation to database")
		return err
	}

	return nil
}

func moveInFlightStateToCurrentState(state *store.ClusterState) error {
	t, err := store.ConvertToGRPCTask(state.InFlight.Task)
	if err != nil {
		return err
	}

	s, err := t.MutableClusters()
	if err != nil {
		return err
	}

	cs, err := store.ConvertFromGRPCClusters(s)
	if err != nil {
		return err
	}

	state.Current = cs
	return nil
}
