package service

import (
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type DeleteLoadBalancerRoles struct {
	State  *spec.UpdateV2_State
	Delete *spec.UpdateV2_DeleteLoadBalancerRoles
}

func deleteLoadBalancerRoles(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteLoadBalancerRoles,
	tracker Tracker,
) {
	// Currently there is no special mechanism for just deleting the
	// roles of the loadbalancer, thus simply just remove them from the
	// state and reconcile the cluster, on failures don't report any
	// partial state.
	idx := clusters.IndexLoadbalancerByIdV2(action.Delete.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't delete roles for loadbalancer %q that is missing from the received state", action.Delete.Handle)
		return
	}

	current := action.State.LoadBalancers[idx]
	current.Roles = slices.DeleteFunc(current.Roles, func(r *spec.RoleV2) bool {
		return slices.Contains(action.Delete.Roles, r.Name)
	})

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           current,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()
	if err := BuildLoadbalancers(buildLogger, lb); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster after roles deletion")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
