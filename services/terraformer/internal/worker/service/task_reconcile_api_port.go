package service

import (
	"context"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func reconcileApiPort(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Reconciling API port")

	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to reconcile Api port, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	if action.Update.GetClusterApiPort() == nil {
		logger.
			Warn().
			Msgf("Received update task with action %T, while wanting to reconcile Api port, assuming the task was misscheduled, ignoring", action.Update.Delta)
		return
	}

	ctx := context.Background()
	k8s := action.Update.State.K8S
	cluster := kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		ExportPort6443:    action.Update.GetClusterApiPort().Open,
		SpawnProcessLimit: processLimit,
	}

	logger = logger.With().Str("cluster", cluster.Id()).Logger()

	// This should only change rules into the firewall and no other
	// changes should be done by the templates that would invalidate
	// the nodepools that would result in needed to act on the [kubernetes.ErrReconcileAll]
	if err := cluster.ReconcileCommon(ctx, logger); err != nil {
		logger.Err(err).Msg("Failed to reconcile cluster api port")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Api Port for kubernetes cluster successfully reconciled")

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}
