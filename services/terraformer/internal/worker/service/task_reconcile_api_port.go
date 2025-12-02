package service

import (
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
	action, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to reconcile Api port, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	k8s := action.Update.State.K8S
	cluster := kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		ExportPort6443:    action.Update.GetClusterApiPort().GetOpen(),
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()

	if err := BuildK8Scluster(buildLogger, cluster); err != nil {
		logger.Err(err).Msg("Failed to reconcile cluster api port")

		tracker.Diagnostics.Push(err)

		possiblyUpdated := cluster.Cluster
		update := tracker.Result.Update()
		update.Kubernetes(possiblyUpdated)
		update.Commit()

		return
	}

	buildLogger.Info().Msg("Api Port for kubernetes cluster successfully reconciled")

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}
