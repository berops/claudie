package service

import (
	"context"
	"errors"
	"slices"

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

	if cluster.Task == nil || (cluster.Task.Id != work.TaskID) {
		logger.Warn().Msg("Recevied task for cluster does not match, ignoring.")
		acknowledge = true
		return
	}

	stage := cluster.Task.Pipeline[cluster.Task.CurrentStage]
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

	advanceToNextStage(cluster)

	if err := propagateResultToCurrentState(logger, work.Result, cluster); err != nil {
		// Since the function works with successfully loaded database data and
		// also the successfully received work result data, there has to be some kind
		// of malformed or corrupted data, thus we don't acknowledge the received message, halting
		// the workflow.
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
	logger.Error().Msg("Task resulted in an error during the current state")

	var (
		cluster = im.Clusters[work.Cluster]
		stage   = cluster.Task.Pipeline[cluster.Task.CurrentStage]

		isErrorPartial = work.Result.Error.Kind == spec.TaskResult_Error_PARTIAL
		isStageWarn    = stage.Description.ErrorLevel == spec.ErrorLevel_ERROR_WARN.String()
	)

	cluster.State.Status = spec.WorkflowV2_ERROR.String()
	cluster.State.Description = work.Result.Error.Description

	if isErrorPartial {
		if err := propagateResultToCurrentState(logger, work.Result, cluster); err != nil {
			// Since the function works with successfully loaded database data and
			// also the successfully received work result data, there has to be some kind
			// of malformed or corrupted data, thus we don't acknowledge the received message, halting
			// the workflow.
			acknowledge = false
			return
		}
	}

	if isStageWarn {
		advanceToNextStage(cluster)
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

func updateCluster(
	logger zerolog.Logger,
	current *store.Clusters,
	result *spec.TaskResult_Update,
) error {
	panic("todo")
}

func clearCluster(
	logger zerolog.Logger,
	current *store.Clusters,
	result *spec.TaskResult_Clear,
) error {
	if result.Clear.K8S != nil && *result.Clear.K8S {
		current.K8s = nil
		current.LoadBalancers = nil
		return nil
	}

	lbs, err := store.ConvertToGRPCLoadBalancers(current.LoadBalancers)
	if err != nil {
		logger.Err(err).Msg("Unexpected unmarshal error for loadbalancers stored in the database")
		return err
	}

	lbs.Clusters = slices.DeleteFunc(lbs.Clusters, func(lb *spec.LBclusterV2) bool {
		return slices.Contains(result.Clear.LoadBalancersIDs, lb.GetClusterInfo().Id())
	})

	if current.LoadBalancers, err = store.ConvertFromGRPCLoadBalancers(lbs); err != nil {
		logger.Err(err).Msg("Unexpected marshal error for loadbalancers")
		return err
	}

	return nil
}

func advanceToNextStage(state *store.ClusterState) {
	state.Task.CurrentStage += 1
	if int(state.Task.CurrentStage) >= len(state.Task.Pipeline) {
		state.Task = nil
		state.State.Status = spec.WorkflowV2_DONE.String()
		state.State.Description = ""
	}
}

func propagateResultToCurrentState(
	logger zerolog.Logger,
	result *spec.TaskResult,
	state *store.ClusterState,
) error {
	switch result := result.Result.(type) {
	case *spec.TaskResult_Update:
		logger.Debug().Msg("Received [Update] as a result for the task")
		return updateCluster(logger, &state.Current, result)
	case *spec.TaskResult_Clear:
		logger.Debug().Msg("Received [Clear] as a result for the task")
		return clearCluster(logger, &state.Current, result)
	case *spec.TaskResult_None_:
		logger.Debug().Msg("Received [None] as a result for the task, no work to be done.")
		return nil
	default:
		logger.Warn().Msgf("received message with unknown result type %T, ignoring", result)
		return nil
	}
}
