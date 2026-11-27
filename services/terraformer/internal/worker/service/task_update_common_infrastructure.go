package service

import (
	"context"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

// reconcileCommonInfrastructure handles the UPDATE_COMMON_INFRA pass. It only
// reconciles the common infrastructure of the cluster targeted by the task and
// never touches the state files of the nodepools.
//
// For additions and deletions of nodepools the common infrastructure must not replace
// any existing resource, so that the existing nodepools are not affected. For deletions,
// common infrastructure no longer used is removed.
//
// For additions of nodepools nothing is committed into the result on success, as no
// nodes are created, the following UPDATE_NODEPOOLS pass commits the cluster. On a
// failed addition the cluster with the new nodepool is committed, so that the manager
// rolls back the addition by deleting the nodepool. For deletions the preceding
// UPDATE_NODEPOOLS pass already committed the cluster.
func reconcileCommonInfrastructure(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to update common infrastructure, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	state := action.Update.State
	if state == nil || state.K8S == nil {
		logger.Warn().Msg("Update task validation failed, required state of the kuberentes cluster to be present, ignoring")
		return
	}

	var (
		ctx = context.Background()
		err error
	)

	switch delta := action.Update.Delta.(type) {
	case *spec.Update_TfAddK8SNodes:
		add := delta.TfAddK8SNodes.GetNew()
		if add == nil {
			logger.Warn().Msg("Received addition of nodes into an existing nodepool, which does not change the common infrastructure, ignoring")
			return
		}

		// The nodepool is added to the state by the UPDATE_NODEPOOLS pass.
		cluster := kubernetes.K8Scluster{
			ProjectName:       projectName,
			Cluster:           state.K8S,
			ExportPort6443:    clusters.FindLoadBalancerApiServerEndpoint(state.LoadBalancers) == nil,
			SpawnProcessLimit: processLimit,
		}

		logger = logger.With().Str("cluster", cluster.Id()).Logger()
		err = cluster.ReconcileCommonNoReplace(ctx, logger, []*spec.NodePool{add.Nodepool}, nil)
		if err != nil {
			// Commit the cluster with the new nodepool even on failure, as there is
			// no mechanism for tracking partial changes out of the terraform output.
			// The manager then rolls back the addition by deleting the nodepool, which
			// also removes any partially created common infrastructure, with the
			// provider of the nodepool present.
			nps := state.K8S.ClusterInfo.NodePools
			state.K8S.ClusterInfo.NodePools = append(slices.Clone(nps), add.Nodepool)

			update := tracker.Result.Update()
			update.Kubernetes(state.K8S) // deep-clones the cluster
			update.Commit()

			state.K8S.ClusterInfo.NodePools = nps
		}
	case *spec.Update_DeletedK8SNodes_:
		whole := delta.DeletedK8SNodes.GetWhole()
		if whole == nil || whole.Nodepool.GetDynamicNodePool() == nil {
			logger.Warn().Msg("Received deletion that does not remove a dynamic nodepool, which does not change the common infrastructure, ignoring")
			return
		}

		// The state no longer contains the deleted nodepool.
		cluster := kubernetes.K8Scluster{
			ProjectName:       projectName,
			Cluster:           state.K8S,
			ExportPort6443:    clusters.FindLoadBalancerApiServerEndpoint(state.LoadBalancers) == nil,
			SpawnProcessLimit: processLimit,
		}

		logger = logger.With().Str("cluster", cluster.Id()).Logger()
		err = cluster.ReconcileCommonNoReplace(ctx, logger, nil, []*spec.NodePool{whole.Nodepool})
	case *spec.Update_TfAddLoadBalancerNodes:
		add := delta.TfAddLoadBalancerNodes.GetNew()
		if add == nil {
			logger.Warn().Msg("Received addition of nodes into an existing loadbalancer nodepool, which does not change the common infrastructure, ignoring")
			return
		}

		idx := clusters.IndexLoadbalancerById(delta.TfAddLoadBalancerNodes.Handle, state.LoadBalancers)
		if idx < 0 {
			logger.
				Warn().
				Msgf("Can't add nodepool for loadbalancer %q that is missing from the received state", delta.TfAddLoadBalancerNodes.Handle)
			return
		}

		// The nodepool is added to the state by the UPDATE_NODEPOOLS pass.
		lb := loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           state.LoadBalancers[idx],
			SpawnProcessLimit: processLimit,
		}

		logger = logger.With().Str("cluster", lb.Id()).Logger()
		err = lb.ReconcileCommonNoReplace(ctx, logger, []*spec.NodePool{add.Nodepool}, nil)
		if err != nil {
			// Commit the loadbalancer with the new nodepool even on failure, as there is
			// no mechanism for tracking partial changes out of the terraform output.
			// The manager then rolls back the addition by deleting the nodepool, which
			// also removes any partially created common infrastructure, with the
			// provider of the nodepool present.
			nps := lb.Cluster.ClusterInfo.NodePools
			lb.Cluster.ClusterInfo.NodePools = append(slices.Clone(nps), add.Nodepool)

			update := tracker.Result.Update()
			update.Loadbalancers(lb.Cluster) // deep-clones the loadbalancer
			update.Commit()

			lb.Cluster.ClusterInfo.NodePools = nps
		}
	case *spec.Update_TfDeleteLoadBalancerNodes:
		if !delta.TfDeleteLoadBalancerNodes.WithNodePool {
			logger.Warn().Msg("Received deletion of nodes from a loadbalancer nodepool, which does not change the common infrastructure, ignoring")
			return
		}

		// The preceding UPDATE_NODEPOOLS pass removed the nodepool from the state.
		deleted := tracker.Shared.DeletedLoadBalancerNodePool
		if deleted.GetDynamicNodePool() == nil {
			logger.Warn().Msg("Received deletion that does not remove a dynamic loadbalancer nodepool, which does not change the common infrastructure, ignoring")
			return
		}

		idx := clusters.IndexLoadbalancerById(delta.TfDeleteLoadBalancerNodes.Handle, state.LoadBalancers)
		if idx < 0 {
			logger.
				Warn().
				Msgf("Can't delete nodepool for loadbalancer %q that is missing from the received state", delta.TfDeleteLoadBalancerNodes.Handle)
			return
		}

		lb := loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           state.LoadBalancers[idx],
			SpawnProcessLimit: processLimit,
		}

		logger = logger.With().Str("cluster", lb.Id()).Logger()
		err = lb.ReconcileCommonNoReplace(ctx, logger, nil, []*spec.NodePool{deleted})
	case *spec.Update_DeletedLoadBalancerNodes_:
		// Retry of a whole loadbalancer nodepool deletion, after a previous attempt
		// already committed the loadbalancer without the deleted nodepool.
		deleted := delta.DeletedLoadBalancerNodes.GetWhole().GetNodepool()
		if deleted.GetDynamicNodePool() == nil {
			logger.Warn().Msg("Received deletion that does not remove a dynamic loadbalancer nodepool, which does not change the common infrastructure, ignoring")
			return
		}

		idx := clusters.IndexLoadbalancerById(delta.DeletedLoadBalancerNodes.Handle, state.LoadBalancers)
		if idx < 0 {
			logger.
				Warn().
				Msgf("Can't delete nodepool for loadbalancer %q that is missing from the received state", delta.DeletedLoadBalancerNodes.Handle)
			return
		}

		// The state no longer contains the deleted nodepool.
		lb := loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           state.LoadBalancers[idx],
			SpawnProcessLimit: processLimit,
		}

		logger = logger.With().Str("cluster", lb.Id()).Logger()
		err = lb.ReconcileCommonNoReplace(ctx, logger, nil, []*spec.NodePool{deleted})
	case *spec.Update_TfAddLoadBalancerRoles:
		action := AddLoadBalancerRoles{
			State: state,
			Add:   delta.TfAddLoadBalancerRoles,
		}
		addLoadBalancerRoles(logger, projectName, processLimit, action, tracker)
		return
	case *spec.Update_DeleteLoadBalancerRoles_:
		action := DeleteLoadBalancerRoles{
			State:  state,
			Delete: delta.DeleteLoadBalancerRoles,
		}
		deleteLoadBalancerRoles(logger, projectName, processLimit, action, tracker)
		return
	case *spec.Update_TfReplaceRoleExternalSettings:
		action := ReplaceRoleExternalSettings{
			State:   state,
			Replace: delta.TfReplaceRoleExternalSettings,
		}
		replaceRoleExternalSettings(logger, projectName, processLimit, action, tracker)
		return
	default:
		logger.
			Warn().
			Msgf("Received update task with action %T while wanting to update common infrastructure, assuming the task was misscheduled, ignoring", action.Update.Delta)
		return
	}

	if err != nil {
		logger.Err(err).Msg("Failed to reconcile common infrastructure")
		tracker.Diagnostics.Push(err)
	}
}
