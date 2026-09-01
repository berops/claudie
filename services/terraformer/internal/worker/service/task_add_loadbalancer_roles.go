package service

import (
	"context"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type AddLoadBalancerRoles struct {
	State *spec.Update_State
	Add   *spec.Update_TerraformerAddLoadBalancerRoles
}

func addLoadBalancerRoles(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action AddLoadBalancerRoles,
	tracker Tracker,
) {
	logger.Info().Msg("Adding LoadBalancer Roles")

	// Currently there is no special mechanism for just adding the
	// roles of the loadbalancer, thus simply add them to the state
	// and reconcile the cluster.
	idx := clusters.IndexLoadbalancerById(action.Add.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't add roles for loadbalancer %q that is missing from the received state", action.Add.Handle)
		return
	}

	current := action.State.LoadBalancers[idx]
	current.Roles = append(current.Roles, action.Add.Roles...)

	ctx := context.Background()
	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           current,
		SpawnProcessLimit: processLimit,
	}

	logger = logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()

	// This should only add new rules into firewalls and no other
	// changes should be done by the templates that would invalidate
	// the nodepools that would result in needed to act on the [loadbalancer.ErrReconcileAll]
	if err := lb.ReconcileCommon(ctx, logger); err != nil {
		logger.Err(err).Msg("Failed to reconcile cluster after roles addition")
		tracker.Diagnostics.Push(err)
		// Contrary to the deletion process, during the addition if any partial changes
		// take effect we have to report them back, however since there is currently
		// no mechanism for tracking partial changes out of the terraform output
		// commit the whole changes, and let manager work out the diff.
		//
		// fallthrough
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
