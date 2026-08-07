package service

import (
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

type MoveNodePoolToAutoscaled struct {
	State *spec.Update_State
	Move  *spec.Update_TerraformerMoveNodePoolToAutoscaled
}

func moveNodePoolToAutoscaled(
	logger zerolog.Logger,
	action MoveNodePoolToAutoscaled,
	tracker Tracker,
) {
	logger.Info().Msg("Moving NodePool to Autoscaled type")
	// Currently there is no special mechanism for moving a
	// nodepool from fixed sized to autoscaled type.
	//
	// Thus simply add the autoscaled config to the
	// nodepool.

	np := nodepools.FindByName(action.Move.Nodepool, action.State.K8S.ClusterInfo.NodePools)
	if np == nil {
		logger.
			Warn().
			Msgf("Can't move nodepool %q to autoscaled, as its missing from the passed in state", action.Move.Nodepool)
		return
	}

	dyn := np.GetDynamicNodePool()
	if dyn == nil {
		logger.
			Warn().
			Msgf("Can't move nodepool %q to autoscaled, as its not a dynamic nodepool", action.Move.Nodepool)
		return
	}

	dyn.AutoscalerConfig = action.Move.Config

	update := tracker.Result.Update()
	update.Kubernetes(action.State.K8S)
	update.Commit()
}
