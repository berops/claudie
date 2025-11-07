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

	if err := propagateResult(logger, cluster, work.Cluster, work.Result); err != nil {
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
		if err := propagateResult(logger, cluster, work.Cluster, work.Result); err != nil {
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

	if err := moveInFlightStateToCurrentState(logger, state); err != nil {
		logger.Err(err).Msg("Failed to move in flight state as current")
		return err
	}

	state.InFlight = nil
	state.State.Status = spec.WorkflowV2_DONE.String()
	state.State.Description = ""

	logger.Info().Msg("Task successfully finished moving to DONE")
	return nil
}

func propagateUpdateResultForCreate(
	logger zerolog.Logger,
	clusterName string,
	inFlight *spec.CreateV2,
	result *spec.TaskResult_Update,
) {
	if k8s := result.Update.K8S; k8s != nil {
		gotName := k8s.GetClusterInfo().GetName()
		if gotName != clusterName {
			// Under normal circumstances this should never happen, this signals either
			// malformed/corrupted data and/or mistake in the scheduling of tasks.
			// Thus return an error rather than continuing with the merge.
			err := fmt.Errorf("Can't update cluster %q with received cluster %q", clusterName, gotName)
			logger.
				Err(err).
				Msg("Unexpected mismatch in the name of the clusters from the received messagge and current state in the database, ignoring")
			return
		}
		inFlight.K8S = k8s
	}

	var toUpdate spec.LoadBalancersV2
	for _, lb := range result.Update.GetLoadBalancers().GetClusters() {
		toUpdate.Clusters = append(toUpdate.Clusters, lb)
	}
	toUpdate.Clusters = slices.DeleteFunc(toUpdate.Clusters, func(lb *spec.LBclusterV2) bool { return lb.TargetedK8S != clusterName })

	// update existing ones.
	for i := range inFlight.LoadBalancers {
		lb := inFlight.LoadBalancers[i].ClusterInfo.Id()
		for j := range toUpdate.Clusters {
			update := toUpdate.Clusters[j].ClusterInfo.Id()
			if lb == update {
				inFlight.LoadBalancers[i] = toUpdate.Clusters[j]
				break
			}
		}
	}

	// add new ones.
	for i := range toUpdate.Clusters {
		id := toUpdate.Clusters[i].ClusterInfo.Id()
		filter := func(lb *spec.LBclusterV2) bool { return lb.ClusterInfo.Id() == id }
		if !slices.ContainsFunc(inFlight.LoadBalancers, filter) {
			inFlight.LoadBalancers = append(inFlight.LoadBalancers, toUpdate.Clusters[i])
		}
	}
}

func propagateClearResult(
	inFlight *spec.DeleteV2,
	result *spec.TaskResult_Clear,
) {
	lbFilter := func(lb *spec.LBclusterV2) bool {
		return slices.Contains(result.Clear.LoadBalancersIDs, lb.GetClusterInfo().Id())
	}
	switch inFlight := inFlight.GetOp().(type) {
	case *spec.DeleteV2_Clusters_:
		if result.Clear.K8S != nil && *result.Clear.K8S {
			inFlight.Clusters.K8S = nil
			inFlight.Clusters.LoadBalancers = nil
			return
		}
		inFlight.Clusters.LoadBalancers = slices.DeleteFunc(inFlight.Clusters.LoadBalancers, lbFilter)
	case *spec.DeleteV2_Loadbalancers:
		inFlight.Loadbalancers.LoadBalancers = slices.DeleteFunc(inFlight.Loadbalancers.LoadBalancers, lbFilter)
	}
}

func propagateResult(
	logger zerolog.Logger,
	cluster *store.ClusterState,
	clusterName string,
	result *spec.TaskResult,
) error {
	inFlight, err := store.ConvertToGRPCTask(cluster.InFlight.Task)
	if err != nil {
		logger.Err(err).Msg("Failed to unmarshal database representation")
		return err
	}

	switch result := result.Result.(type) {
	case *spec.TaskResult_Update:
		logger.Debug().Msg("Received [Update] as a result for the task")
		create := inFlight.GetCreate()
		if create == nil {
			logger.Warn().Msgf("Received [Update] as a result for scheduled task %T, ignoring", inFlight.GetDo())
			panic("todo, for update maybe ?")
		}
		propagateUpdateResultForCreate(logger, clusterName, create, result)
	case *spec.TaskResult_Clear:
		logger.Debug().Msg("Received [Clear] as a result for the task")
		delete := inFlight.GetDelete()
		if delete == nil {
			logger.Warn().Msgf("Received [Clear] as a result for scheduled task %T, ignoring", inFlight.GetDo())
			break
		}
		propagateClearResult(delete, result)
	case *spec.TaskResult_None_:
		logger.Debug().Msg("Received [None] as a result for the task, no work to be done.")
	default:
		logger.Warn().Msgf("received message with unknown result type %T, ignoring", result)
	}

	cluster.InFlight.Task, err = store.ConvertFromGRPCTask(inFlight)
	if err != nil {
		logger.Err(err).Msg("Failed to marshal grpc representation to database")
		return err
	}

	return nil
}

func moveCreateToCurrentState(inFlight *spec.TaskV2_Create, current *spec.ClustersV2) {
	current.K8S = inFlight.Create.K8S
	current.LoadBalancers.Clusters = inFlight.Create.LoadBalancers
}

func moveDeleteToCurrentState(inFlight *spec.TaskV2_Delete, current *spec.ClustersV2) {
	switch inFlight := inFlight.Delete.GetOp().(type) {
	case *spec.DeleteV2_Clusters_:
		current.K8S = inFlight.Clusters.K8S
		current.LoadBalancers.Clusters = inFlight.Clusters.LoadBalancers
	case *spec.DeleteV2_Loadbalancers:
		current.LoadBalancers.Clusters = inFlight.Loadbalancers.LoadBalancers
	}
}

func moveInFlightStateToCurrentState(
	logger zerolog.Logger,
	state *store.ClusterState,
) error {
	s, err := store.ConvertToGRPCClusterState(state)
	if err != nil {
		return err
	}

	switch inFlight := s.Task.Task.GetDo().(type) {
	case *spec.TaskV2_Create:
		logger.Debug().Msg("Moving [Create] into the current state of the cluster")
		moveCreateToCurrentState(inFlight, s.Current)
	case *spec.TaskV2_Delete:
		logger.Debug().Msg("Moving [Delete] into the current state of the cluster")
		moveDeleteToCurrentState(inFlight, s.Current)
	default:
		logger.Warn().Msgf("Received message with unknown result type %T, ignoring", inFlight)
	}

	k, err := store.ConvertFromGRPCClusterState(s)
	if err != nil {
		return err
	}

	*state = *k
	return nil
}
