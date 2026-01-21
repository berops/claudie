package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/uuid"
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

// Process the Message from the NATS message queue.
//
// Note that NATS follows the at least once delivery, thus messages may be re-delivered
// could be due to ack failing or various reasons. The 'acknowledge' return parameter
// tells whether the message should be ack or not. If not the message will be 100%
// re-delivered again, otherwise the message will be ack from the message queue.
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
		// Make sure the message is retried, in case of DB failures.
		acknowledge = false

		// Propagating the result failed for unknown reasons, could be due
		// to malformed message or simply invalid message changes that the
		// manager refuses to merge. How to proceed here is to acknowledge
		// the 'bad' message, but without any advancing to the next stage
		// or persisting the changes, and let manager reconciliation take
		// over the changes again by rescheduling the same 'task' but under
		// new ID.
		//
		// Once rescheduled under a new ID, possible re-delivery of the old
		// message will be discarded as guarded by the above [work.TaskID]
		// check, and won't be considered for the newly rescheduled task
		// under the new ID.
		//
		// Also re-scheduling under a new UUID is necessary to avoid
		// catching duplicate messages within NATS itself.

		newUUID := uuid.New().String()

		logger.
			Warn().
			Msgf(
				"Rescheduling task %q, scheduling under new ID %q, due to failed result propagation of received message: %v",
				cluster.InFlight.Id,
				newUUID,
				err,
			)

		cluster.InFlight.Id = newUUID
		cluster.State = store.Workflow{
			Status: spec.Workflow_WAIT_FOR_PICKUP.String(),
		}

		if err := stores.store.UpdateConfig(ctx, im); err != nil {
			// In both cases, retry processing the message.
			if errors.Is(err, store.ErrNotFoundOrDirty) {
				logger.Warn().Msg("Failed to update InputManifest, dirty write")
			} else {
				logger.Err(err).Msg("Failed to update InputManifest")
			}
			acknowledge = false
			return
		}

		logger.
			Info().
			Msgf("Successfully rescheduled task under new ID %q", newUUID)

		// If the Ack messge fails here the task was rescheduling under a new ID thus processing
		// the result again, will be discarded before reaching any processing of the message.
		acknowledge = true
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
			logger.Warn().Msg("Failed to update InputManifest, dirty write")
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

	cluster.State.Status = spec.Workflow_ERROR.String()
	cluster.State.Description = work.Result.Error.Description

	if isErrorPartial {
		if err := propagateResult(logger, cluster, work.Result); err != nil {
			// Make sure the message is retried, in case of DB failures.
			acknowledge = false

			// Propagating the result failed for unknown reasons, could be due
			// to malformed message or simply invalid message changes that the
			// manager refuses to merge. How to proceed here is to acknowledge
			// the 'bad' message, but without any advancing to the next stage
			// or persisting the changes, and let manager reconciliation take
			// over the changes again by rescheduling the same 'task' but under
			// new ID.
			//
			// Once rescheduled under a new ID, possible re-delivery of the old
			// message will be discarded as guarded by the above [work.TaskID]
			// check in the [ProcessTask] function, and won't be considered for
			// the newly rescheduled task under the new ID.
			//
			// Also re-scheduling under a new UUID is necessary to avoid
			// catching duplicate messages within NATS itself.

			newUUID := uuid.New().String()

			logger.
				Warn().
				Msgf(
					"Rescheduling task %q, scheduling under new ID %q, due to failed result propagation of received message: %v",
					cluster.InFlight.Id,
					newUUID,
					err,
				)

			cluster.InFlight.Id = newUUID
			cluster.State = store.Workflow{
				Status: spec.Workflow_WAIT_FOR_PICKUP.String(),
			}

			if err := stores.store.UpdateConfig(ctx, im); err != nil {
				// In both cases, retry processing the message.
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msg("Failed to update InputManifest, dirty write")
				} else {
					logger.Err(err).Msg("Failed to update InputManifest")
				}
				acknowledge = false
				return
			}

			logger.
				Info().
				Msgf("Successfully rescheduled task under new ID %q", newUUID)

			// If the Ack messge fails here the task was rescheduling under a new ID thus processing
			// the result again, will be discarded before reaching any processing of the message.
			acknowledge = true
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
			logger.Warn().Msg("Failed to update InputManifest, dirty write")
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

		state.State.Status = spec.Workflow_WAIT_FOR_PICKUP.String()
		state.State.Description = state.InFlight.Pipeline[state.InFlight.CurrentStage].Description.About
		return nil
	}

	if err := moveInFlightStateToCurrentState(state); err != nil {
		return err
	}

	state.InFlight = nil
	state.State.Status = spec.Workflow_DONE.String()
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
				Warn().
				Msgf("Failed to consume update result: %v", err)
			return err
		}
	case *spec.TaskResult_Clear:
		logger.Debug().Msg("Received [Clear] as a result for the task")
		if err := inFlight.Task.ConsumeClearResult(result); err != nil {
			logger.
				Warn().
				Msgf("Failed to consume clear result: %v", err)
			return err
		}
	case *spec.TaskResult_None_:
		logger.Debug().Msg("Received [None] as a result for the task, no work to be done.")
	default:
		logger.Warn().Msgf("Received message with unknown result type %T, ignoring", result)
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
