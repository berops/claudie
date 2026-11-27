package service

import (
	"context"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

// reconcileNodePoolsInfrastructure handles the UPDATE_NODEPOOLS pass. It only
// reconciles the state files of the nodepools targeted by the task, while only
// reading the common infrastructure of the cluster.
//
// Additions only reconcile the added nodepool and deletions only destroy the deleted
// nodes or nodepool. Deletions are idempotent, so that a replayed pass is a no-op for
// infrastructure already destroyed.
func reconcileNodePoolsInfrastructure(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to update nodepools, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	state := action.Update.State
	if state == nil || state.K8S == nil {
		logger.Warn().Msg("Update task validation failed, required state of the kuberentes cluster to be present, ignoring")
		return
	}

	switch delta := action.Update.Delta.(type) {
	case *spec.Update_TfAddK8SNodes:
		addKubernetesNodes(logger, projectName, processLimit, state, delta.TfAddK8SNodes, tracker)
	case *spec.Update_DeletedK8SNodes_:
		deleteKubernetesNodes(logger, stores, projectName, processLimit, state, delta.DeletedK8SNodes, tracker)
	case *spec.Update_TfAddLoadBalancerNodes:
		addLoadBalancerNodes(logger, projectName, processLimit, state, delta.TfAddLoadBalancerNodes, tracker)
	case *spec.Update_TfDeleteLoadBalancerNodes:
		deleteLoadBalancerNodes(logger, stores, projectName, processLimit, state, delta.TfDeleteLoadBalancerNodes, tracker)
	case *spec.Update_DeletedLoadBalancerNodes_:
		deletedLoadBalancerNodes(logger, stores, projectName, processLimit, state, delta.DeletedLoadBalancerNodes, tracker)
	default:
		logger.
			Warn().
			Msgf("Received update task with action %T while wanting to update nodepools, assuming the task was misscheduled, ignoring", action.Update.Delta)
	}
}
func addKubernetesNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	state *spec.Update_State,
	add *spec.Update_TerraformerAddK8SNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Adding kubernetes Nodes")

	var (
		ctx     = context.Background()
		k8s     = state.K8S
		cluster = kubernetes.K8Scluster{
			ProjectName:       projectName,
			Cluster:           k8s,
			ExportPort6443:    clusters.FindLoadBalancerApiServerEndpoint(state.LoadBalancers) == nil,
			SpawnProcessLimit: processLimit,
		}
		target string
	)

	logger = logger.With().Str("cluster", cluster.Id()).Logger()

	switch kind := add.Kind.(type) {
	case *spec.Update_TerraformerAddK8SNodes_Existing_:
		np := nodepools.FindByName(kind.Existing.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't add nodes to nodepool %q of kubernetes cluster %q as the nodepool is missing from the received state",
					kind.Existing.Nodepool,
					k8s.ClusterInfo.Id(),
				)
			return
		}

		if np.GetStaticNodePool() != nil {
			// Static nodes are not, and should be not, added through the
			// terraformer stage, thus here we can only focus on considering that
			// the nodes to be added here are dynamic nodes.
			logger.
				Warn().
				Msgf(
					"Can't work with static nodes from nodepool %q within kubernetes cluster %q, as their infrastructure cannot be managed by claudie, ignoring",
					np.Name,
					k8s.ClusterInfo.Id(),
				)
			return
		}

		nodepools.AppendDynamicNodes(np, kind.Existing.Nodes)
		target = np.Name
	case *spec.Update_TerraformerAddK8SNodes_New_:
		k8s.ClusterInfo.NodePools = append(k8s.ClusterInfo.NodePools, kind.New.Nodepool)
		target = kind.New.Nodepool.Name
	default:
		logger.
			Warn().
			Msgf("Received add nodes to kuberentes action, but with an invalid addition kind %T, ignoring", kind)
		return
	}

	if err := cluster.NodePools(ctx, logger, target); err != nil {
		logger.Err(err).Msgf("Failed to reconcile new nodes for nodepool %q", target)
		tracker.Diagnostics.Push(err)
		// since there currently is no mechanism for tracking partial changes out
		// of the terraform output, commit the changes and let manager work out the diff.
		//
		// fallthrough
	}

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}

func deleteKubernetesNodes(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	state *spec.Update_State,
	del *spec.Update_DeletedK8SNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Deleting Kubernetes Nodes")

	// The deletion of the nodes for the kubernetes cluster is handled by the
	// kuber service, in here we only destroy the spawned infrastructure for the
	// dynamic nodepools.
	//
	// The state has already been modified and does not include the deleted nodes.

	var (
		ctx     = context.Background()
		cluster = kubernetes.K8Scluster{
			ProjectName:       projectName,
			Cluster:           state.K8S,
			ExportPort6443:    clusters.FindLoadBalancerApiServerEndpoint(state.LoadBalancers) == nil,
			SpawnProcessLimit: processLimit,
		}
	)

	logger = logger.With().Str("cluster", cluster.Id()).Logger()

	switch kind := del.Kind.(type) {
	case *spec.Update_DeletedK8SNodes_Partial_:
		if err := cluster.NodePools(ctx, logger, kind.Partial.Nodepool); err != nil {
			logger.Err(err).Msgf("Failed to destroy infrastructure for removed nodes of nodepool %q", kind.Partial.Nodepool)
			tracker.Diagnostics.Push(err)
			return
		}
	case *spec.Update_DeletedK8SNodes_Whole:
		if kind.Whole.Nodepool.GetDynamicNodePool() == nil {
			// Nothing to delete for static nodepools.
			return
		}

		if err := cluster.DestroyNodePoolOnly(ctx, logger, kind.Whole.Nodepool, stores.s3); err != nil {
			logger.Err(err).Msgf("Failed to destroy nodepool %q", kind.Whole.Nodepool.Name)
			tracker.Diagnostics.Push(err)
			return
		}
	default:
		logger.
			Warn().
			Msgf("Received delete nodes for kuberentes, but with an invalid kind %T, ignoring", kind)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}

func addLoadBalancerNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	state *spec.Update_State,
	add *spec.Update_TerraformerAddLoadBalancerNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Adding LoadBalancer Nodes")

	idx := clusters.IndexLoadbalancerById(add.Handle, state.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't add nodes for loadbalancer %q that is missing from the received state", add.Handle)
		return
	}

	var (
		ctx     = context.Background()
		current = state.LoadBalancers[idx]
		lb      = loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           current,
			SpawnProcessLimit: processLimit,
		}
		target string
	)

	logger = logger.With().Str("cluster", lb.Id()).Logger()

	switch kind := add.Kind.(type) {
	case *spec.Update_TerraformerAddLoadBalancerNodes_Existing_:
		np := nodepools.FindByName(kind.Existing.Nodepool, current.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't add nodes to nodepool %q as the nodepool is missing from the received state",
					kind.Existing.Nodepool,
				)
			return
		}

		// Append new nodes only for dynamic nodepools.
		//
		// Static nodepools node additions are not handled
		// by terraformer at all.
		nodepools.AppendDynamicNodes(np, kind.Existing.Nodes)
		target = np.Name
	case *spec.Update_TerraformerAddLoadBalancerNodes_New_:
		current.ClusterInfo.NodePools = append(current.ClusterInfo.NodePools, kind.New.Nodepool)
		target = kind.New.Nodepool.Name
	default:
		logger.
			Warn().
			Msgf("Received add nodes to loadbalancers action, but with an invalid addition kind %T, ignoring", kind)
		return
	}

	if err := lb.NodePools(ctx, logger, target); err != nil {
		logger.Err(err).Msgf("Failed to reconcile new nodes for nodepool %q", target)
		tracker.Diagnostics.Push(err)
		// since there currently is no mechanism for tracking partial changes out
		// of the terraform output, commit the changes and let manager work out the diff.
		//
		// fallthrough
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}

func deleteLoadBalancerNodes(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	state *spec.Update_State,
	del *spec.Update_TerraformerDeleteLoadBalancerNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Deleting LoadBalancer Nodes")

	idx := clusters.IndexLoadbalancerById(del.Handle, state.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't delete nodes for loadbalancer %q that is missing from the received state", del.Handle)
		return
	}

	var (
		ctx     = context.Background()
		current = state.LoadBalancers[idx]
		lb      = loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           current,
			SpawnProcessLimit: processLimit,
		}
	)

	logger = logger.With().Str("cluster", lb.Id()).Logger()

	if del.WithNodePool {
		deleted := nodepools.FindByName(del.Nodepool, current.ClusterInfo.NodePools)
		if deleted == nil {
			logger.
				Warn().
				Msgf("Can't delete nodepool %q as the nodepool is missing from the received state", del.Nodepool)
			return
		}

		// The nodepool is removed from the state and remembered for the following
		// UPDATE_COMMON_INFRA pass, which will no longer find it in the state.
		current.ClusterInfo.NodePools = nodepools.DeleteByName(current.ClusterInfo.NodePools, del.Nodepool)
		tracker.Shared.DeletedLoadBalancerNodePool = deleted

		if err := lb.DestroyNodePoolOnly(ctx, logger, deleted, stores.s3); err != nil {
			logger.Err(err).Msgf("Failed to destroy nodepool %q", deleted.Name)
			tracker.Diagnostics.Push(err)
			return
		}
	} else {
		np := nodepools.FindByName(del.Nodepool, current.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't delete nodes from nodepool %q as the nodepool is missing from the received state",
					del.Nodepool,
				)
			return
		}

		nodepools.DeleteNodes(np, del.Nodes)
		if err := lb.NodePools(ctx, logger, del.Nodepool); err != nil {
			logger.Err(err).Msgf("Failed to destroy nodes in nodepool %q", del.Nodepool)
			tracker.Diagnostics.Push(err)
			return
		}
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}

// deletedLoadBalancerNodes handles a retried deletion of a whole loadbalancer nodepool.
//
// When a pass after the first UPDATE_NODEPOOLS pass of the deletion fails, the result
// already contains the loadbalancer without the deleted nodepool. The manager consumes
// it and rewrites the delta to [spec.Update_DeletedLoadBalancerNodes], carrying the
// deleted nodepool, with which the stage is then retried. The state of the loadbalancer
// no longer contains the deleted nodepool.
func deletedLoadBalancerNodes(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	state *spec.Update_State,
	del *spec.Update_DeletedLoadBalancerNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Deleting LoadBalancer Nodes")

	deleted := del.GetWhole().GetNodepool()
	if deleted == nil {
		logger.
			Warn().
			Msgf("Received deleted nodes of loadbalancer %q without a whole nodepool, which is not handled by the terraformer, ignoring", del.Handle)
		return
	}

	idx := clusters.IndexLoadbalancerById(del.Handle, state.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't delete nodes for loadbalancer %q that is missing from the received state", del.Handle)
		return
	}

	var (
		ctx = context.Background()
		lb  = loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           state.LoadBalancers[idx],
			SpawnProcessLimit: processLimit,
		}
	)

	logger = logger.With().Str("cluster", lb.Id()).Logger()

	if err := lb.DestroyNodePoolOnly(ctx, logger, deleted, stores.s3); err != nil {
		logger.Err(err).Msgf("Failed to destroy nodepool %q", deleted.Name)
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
