package service

import (
	"context"
	"errors"
	"fmt"
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

	advanceToNextStage(logger, cluster)

	if err := propagateResultToCurrentState(logger, work.Cluster, work.Result, cluster); err != nil {
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
	logger.
		Error().
		Msgf("Task resulted in an error during the current state: %s",
			work.Result.Error.Description,
		)

	var (
		cluster = im.Clusters[work.Cluster]
		stage   = cluster.Task.Pipeline[cluster.Task.CurrentStage]

		isErrorPartial = work.Result.Error.Kind == spec.TaskResult_Error_PARTIAL
		isStageWarn    = stage.Description.ErrorLevel == spec.ErrorLevel_ERROR_WARN.String()
	)

	cluster.State.Status = spec.WorkflowV2_ERROR.String()
	cluster.State.Description = work.Result.Error.Description

	if isErrorPartial {
		if err := propagateResultToCurrentState(logger, work.Cluster, work.Result, cluster); err != nil {
			// Since the function works with successfully loaded database data and
			// also the successfully received work result data, there has to be some kind
			// of malformed or corrupted data, thus we don't acknowledge the received message, halting
			// the workflow.
			acknowledge = false
			return
		}
	}

	if isStageWarn {
		advanceToNextStage(logger, cluster)
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
	clusterName string,
	current *store.Clusters,
	result *spec.TaskResult_Update,
) error {
	if k8s := result.Update.K8S; k8s != nil {
		gotName := k8s.GetClusterInfo().GetName()
		if gotName != clusterName {
			// Under normal circumstances this should never happen, this signals either
			// malformed/corrupted data and/or mistake in the scheduling of tasks.
			// Thus return an error rather than continuing with the merge.
			err := fmt.Errorf("Can't update cluster %q with received cluster %q", clusterName, gotName)
			logger.
				Err(err).
				Msg("Unexpected mismatch in the name of the clusters from the received messagge and current state in the database")
			return err
		}

		b, err := store.ConvertFromGRPCCluster(k8s)
		if err != nil {
			logger.Err(err).Msg("Unexpected marshal error for the received updated kubernetes cluster")
			return err
		}

		current.K8s = b
	}

	dbLbs, err := store.ConvertToGRPCLoadBalancers(current.LoadBalancers)
	if err != nil {
		logger.Err(err).Msg("Unexpected unmarshal error for loadbalancers loaded from database")
		return err
	}

	var toUpdate spec.LoadBalancersV2
	for _, lb := range result.Update.GetLoadBalancers().GetClusters() {
		toUpdate.Clusters = append(toUpdate.Clusters, lb)
	}
	toUpdate.Clusters = slices.DeleteFunc(toUpdate.Clusters, func(lb *spec.LBclusterV2) bool { return lb.TargetedK8S != clusterName })

	// update existing ones.
	for i := range dbLbs.Clusters {
		db := dbLbs.Clusters[i].ClusterInfo.Id()
		for j := range toUpdate.Clusters {
			update := toUpdate.Clusters[j].ClusterInfo.Id()
			if db == update {
				dbLbs.Clusters[i] = toUpdate.Clusters[j]
				break
			}
		}
	}

	// add new ones.
	for i := range toUpdate.Clusters {
		id := toUpdate.Clusters[i].ClusterInfo.Id()
		filter := func(lb *spec.LBclusterV2) bool { return lb.ClusterInfo.Id() == id }
		if !slices.ContainsFunc(dbLbs.Clusters, filter) {
			dbLbs.Clusters = append(dbLbs.Clusters, toUpdate.Clusters[i])
		}
	}

	b, err := store.ConvertFromGRPCLoadBalancers(dbLbs)
	if err != nil {
		logger.Err(err).Msg("Unexpected marshal error for loadbalancers loaded from database")
		return err
	}

	current.LoadBalancers = b
	return nil
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

func advanceToNextStage(logger zerolog.Logger, state *store.ClusterState) {
	state.Task.CurrentStage += 1

	if int(state.Task.CurrentStage) >= len(state.Task.Pipeline) {
		logger.Info().Msg("Task successfully finished moving to DONE")
		state.Task = nil
		state.State.Status = spec.WorkflowV2_DONE.String()
		state.State.Description = ""
	} else {
		logger.
			Info().
			Msgf("Advancing task to the next stage %s",
				state.Task.Pipeline[state.Task.CurrentStage].Kind,
			)
		state.State.Status = spec.WorkflowV2_WAIT_FOR_PICKUP.String()
		state.State.Description = state.Task.Pipeline[state.Task.CurrentStage].Description.About
	}
}

func propagateResultToCurrentState(
	logger zerolog.Logger,
	clusterName string,
	result *spec.TaskResult,
	state *store.ClusterState,
) error {
	switch result := result.Result.(type) {
	case *spec.TaskResult_Update:
		logger.Debug().Msg("Received [Update] as a result for the task")
		return updateCluster(logger, clusterName, &state.Current, result)
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
