package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func reconcileInfrastructure(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	action, ok := task.GetDo().(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to update infrastructure, assuming the task was misscheduled, ignoring", task.GetDo())
		return
	}

	state := action.Update.State
	if state == nil || state.K8S == nil {
		logger.Warn().Msg("Update task validation failed, required state of the kuberentes cluster to be present, ignoring")
		return
	}

	switch delta := action.Update.Delta.(type) {
	case *spec.UpdateV2_DeleteLoadBalancer_:
		id := delta.DeleteLoadBalancer.Id
		destroyLoadBalancer(logger, projectName, id, state.LoadBalancers, processLimit, stores, tracker)
	case *spec.UpdateV2_AddLoadBalancer_:
		lb := delta.AddLoadBalancer.LoadBalancer
		reconcileLoadBalancer(logger, projectName, processLimit, lb, tracker)
	case *spec.UpdateV2_ReconcileLoadBalancer_:
		lb := delta.ReconcileLoadBalancer.LoadBalancer
		reconcileLoadBalancer(logger, projectName, processLimit, lb, tracker)
	case *spec.UpdateV2_ReplaceDns_:
		dns := delta.ReplaceDns
		replaceDns(logger, projectName, processLimit, state, dns, tracker)
	default:
		logger.
			Warn().
			Msgf("Received update task with action %T, assuming the task was misscheduled, ignoring", action.Update.Delta)
		return
	}
}

func reconcileApiPort(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	action, ok := task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to reconcile Api port, assuming the task was misscheduled, ignoring", task.Do)
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
		tracker.Diagnostics.Push(err.Error())

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

func destroyLoadBalancer(
	logger zerolog.Logger,
	projectName string,
	toDestroy string,
	lbs []*spec.LBclusterV2,
	processLimit *semaphore.Weighted,
	stores Stores,
	tracker Tracker,
) {
	idx := clusters.IndexLoadbalancerByIdV2(toDestroy, lbs)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Update task validation failed, required loadbalancer to delete %q to be present, ignoring", toDestroy)
		return
	}

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           lbs[idx],
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()
	if err := DestroyCluster(buildLogger, projectName, &lb, stores.s3, stores.dynamo); err != nil {
		tracker.Diagnostics.Push(err.Error())
		return
	}

	clear := tracker.Result.Clear()
	clear.LoadBalancers(lb.Cluster.ClusterInfo.Id())
	clear.Commit()
}

func reconcileLoadBalancer(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	toReconcile *spec.LBclusterV2,
	tracker Tracker,
) {
	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           toReconcile,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()
	if err := BuildLoadbalancers(buildLogger, lb); err != nil {
		// Some part of the loadbalancer infrastructure was not build successfully.
		// Since we still want to report the partially build infrastructure back to the
		// caller, fallthrough here.
		tracker.Diagnostics.Push(err.Error())
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}

func replaceDns(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	state *spec.UpdateV2_State,
	delta *spec.UpdateV2_ReplaceDns,
	tracker Tracker,
) {
	idx := clusters.IndexLoadbalancerByIdV2(delta.LoadBalancerId, state.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't replace DNS for loadbalancer %q that is missing from the received state", delta.LoadBalancerId)
		return
	}

	lb := state.LoadBalancers[idx]
	if lb.Dns != nil {
		dns := loadbalancer.DNS{
			ProjectName:       projectName,
			ClusterName:       lb.ClusterInfo.Name,
			ClusterHash:       lb.ClusterInfo.Hash,
			NodeIPs:           nodepools.PublicEndpoints(lb.ClusterInfo.NodePools),
			Dns:               lb.Dns,
			SpawnProcessLimit: processLimit,
		}

		if err := dns.DestroyDNSRecords(logger); err != nil {
			logger.Err(err).Msg("Failed to destroy DNS records")
			tracker.Diagnostics.Push(err.Error())
			return
		}

		lb.Dns = nil
	}

	if delta.Dns == nil {
		update := tracker.Result.Update()
		update.Loadbalancers(lb)
		update.Commit()
		return
	}

	lb.Dns = delta.Dns
	dns := loadbalancer.DNS{
		ProjectName:       projectName,
		ClusterName:       lb.ClusterInfo.Name,
		ClusterHash:       lb.ClusterInfo.Hash,
		NodeIPs:           nodepools.PublicEndpoints(lb.ClusterInfo.NodePools),
		Dns:               lb.Dns,
		SpawnProcessLimit: processLimit,
	}

	if err := dns.CreateDNSRecords(logger); err != nil {
		logger.Err(err).Msg("Failed to create new DNS records")
		tracker.Diagnostics.Push(err.Error())
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb)
	update.Commit()
}
