package service

import (
	"context"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type DeleteLoadBalancerRoles struct {
	State  *spec.Update_State
	Delete *spec.Update_DeleteLoadBalancerRoles
}

func deleteLoadBalancerRoles(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteLoadBalancerRoles,
	tracker Tracker,
) {
	logger.Info().Msg("Deleting LoadBalancer Roles")

	// Currently there is no special mechanism for just deleting the
	// roles of the loadbalancer, thus simply just remove them from the
	// state and reconcile the cluster, on failures don't report any
	// partial state.
	idx := clusters.IndexLoadbalancerById(action.Delete.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't delete roles for loadbalancer %q that is missing from the received state", action.Delete.Handle)
		return
	}

	current := action.State.LoadBalancers[idx]
	current.Roles = slices.DeleteFunc(current.Roles, func(r *spec.Role) bool {
		return slices.Contains(action.Delete.Roles, r.Name)
	})

	ctx := context.Background()
	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           current,
		SpawnProcessLimit: processLimit,
	}

	logger = logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()

	// This should only delete new rules into firewalls and no other
	// changes should be done by the templates that would invalidate
	// the nodepools that would result in needed to act on the [loadbalancer.ErrReconcileAll]
	if err := lb.ReconcileCommon(ctx, logger); err != nil {
		logger.Err(err).Msg("Failed to reconcile cluster after roles deletion")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
