package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func build(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	action, ok := task.GetDo().(*spec.TaskV2_Create)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to create new infrastructure, assuming the task was misscheduled, ignoring", task.GetDo())
		return
	}

	k8s := action.Create.K8S
	lbs := action.Create.LoadBalancers

	if k8s == nil {
		logger.
			Warn().
			Msg("create task validation failed, required desired state of the kuberentes cluster to be present, ignoring")
		return
	}

	cluster := kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		ExportPort6443:    clusters.FindAssignedLbApiEndpointV2(lbs) == nil,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()

	if err := BuildK8Scluster(buildLogger, cluster); err != nil {
		tracker.Diagnostics.Push(err.Error())

		possiblyUpdated := k8s
		update := tracker.Result.Update()
		update.Kubernetes(possiblyUpdated)
		update.Commit()

		return
	}

	buildLogger.Info().Msg("Infrastructure for kubernetes cluster build successfully")

	var loadbalancers []loadbalancer.LBcluster
	for _, lb := range lbs {
		loadbalancers = append(loadbalancers, loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           lb,
			SpawnProcessLimit: processLimit,
		})
	}

	err := concurrent.Exec(loadbalancers, func(_ int, cluster loadbalancer.LBcluster) error {
		buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
		return BuildLoadbalancers(buildLogger, cluster)
	})
	if err != nil {
		// Some part of loadbalancer infrastructure was not build successfully.
		// Since we still want to report the partially build infrastructure back to the
		// caller we fallthrough here.
		tracker.Diagnostics.Push(err.Error())
	}

	var (
		updatedK8s                   = k8s
		possiblyUpdatedLoadBalancers []*spec.LBclusterV2
	)

	for _, lb := range loadbalancers {
		possiblyUpdatedLoadBalancers = append(possiblyUpdatedLoadBalancers, lb.Cluster)
	}

	update := tracker.Result.Update()
	update.Kubernetes(updatedK8s)
	update.Loadbalancers(possiblyUpdatedLoadBalancers...)
	update.Commit()
}

// Builds the required infrastructure by looking at the difference between
// the current and desired state based on the passed in [kubernetes.K8Scluster].
// On success updates the [kubernetes.K8Scluster.CurrentState] to the desired state.
// On failure, any desred infra is reverted back to current.
func BuildK8Scluster(logger zerolog.Logger, state kubernetes.K8Scluster) error {
	logger.Info().Msg("Creating infrastructure")

	if err := state.Build(logger); err != nil {
		logger.Err(err).Msg("failed to build cluster")
		return err
	}

	logger.Info().Msg("Cluster build successfully")
	return nil
}

// Builds the required infrastructure by looking at the difference between
// the current and desired state based on the passed in [loadbalancer.LBcluster].
// On success updates the [loadbalancer.LBcluster.CurrentState] to the desired state.
// On failure, any desred infra is reverted back to current.
func BuildLoadbalancers(logger zerolog.Logger, state loadbalancer.LBcluster) error {
	logger.Info().Msg("Creating loadbalancer infrastructure")

	if err := state.Build(logger); err != nil {
		logger.Err(err).Msg("failed to build cluster")
		return err
	}

	logger.Info().Msg("Loadbalancer infrastructure successfully created")
	return nil
}
