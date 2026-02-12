package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type ReplaceRoleExternalSettings struct {
	State   *spec.Update_State
	Replace *spec.Update_TerraformerReplaceRoleExternalSettings
}

func replaceRoleExternalSettings(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action ReplaceRoleExternalSettings,
	tracker Tracker,
) {
	logger.Info().Msg("Replacing Role external settings")

	idx := clusters.IndexLoadbalancerById(action.Replace.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't replace role settings for loadbalanacer %q that is missing from the received state", action.Replace.Handle)
		return
	}

	current := action.State.LoadBalancers[idx]

	var toEdit *spec.Role
	for _, role := range current.Roles {
		if role.Name == action.Replace.Role {
			toEdit = role
			break
		}
	}

	if toEdit == nil {
		logger.
			Warn().
			Msgf("Role %q for loadbalancer %q is missing from received state", action.Replace.Role, action.Replace.Handle)
		return
	}

	toEdit.Port = action.Replace.Port
	toEdit.Protocol = action.Replace.Protocol
	toEdit.RoleType = action.Replace.RoleType

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           current,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()
	if err := BuildLoadbalancers(buildLogger, lb); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster after role editing")
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
