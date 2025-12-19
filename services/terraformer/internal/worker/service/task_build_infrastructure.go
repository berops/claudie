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
	tracker Tracker,
) {
	action, ok := tracker.Task.Do.(*spec.Task_Create)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to create new infrastructure, assuming the task was misscheduled, ignoring", tracker.Task.Do)
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
		ExportPort6443:    clusters.FindAssignedLbApiEndpoint(lbs) == nil,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()

	if err := BuildK8Scluster(buildLogger, cluster); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster")

		tracker.Diagnostics.Push(err)

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
		logger.Err(err).Msg("Failed to reconcile loadbalancers")
		// Some part of loadbalancer infrastructure was not build successfully.
		// Since we still want to report the partially build infrastructure back to the
		// caller we fallthrough here.
		tracker.Diagnostics.Push(err)
	}

	var (
		updatedK8s                   = k8s
		possiblyUpdatedLoadBalancers []*spec.LBcluster
	)

	for _, lb := range loadbalancers {
		possiblyUpdatedLoadBalancers = append(possiblyUpdatedLoadBalancers, lb.Cluster)
	}

	update := tracker.Result.Update()
	update.Kubernetes(updatedK8s)
	update.Loadbalancers(possiblyUpdatedLoadBalancers...)
	update.Commit()
}
