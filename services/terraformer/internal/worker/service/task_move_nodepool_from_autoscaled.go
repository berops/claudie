package service

import (
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

type MoveNodePoolFromAutoscaled struct {
	State *spec.Update_State
	Move  *spec.Update_TerraformerMoveNodePoolFromAutoscaled
}

func moveNodePoolFromAutoscaled(
	logger zerolog.Logger,
	action MoveNodePoolFromAutoscaled,
	tracker Tracker,
) {
	// Currently there is no special mechanism for moving a
	// nodepool from autoscaled to fixed type.
	//
	// Thus simply remove the autoscaled config.

	np := nodepools.FindByName(action.Move.Nodepool, action.State.K8S.ClusterInfo.NodePools)
	if np == nil {
		logger.
			Warn().
			Msgf("Can't move nodepool %q to fixed sized, as its missing from the passed in state", action.Move.Nodepool)
		return
	}

	dyn := np.GetDynamicNodePool()
	if dyn == nil {
		logger.
			Warn().
			Msgf("Can't move nodepool %q to fixed size, as its not a dynamic nodepool", action.Move.Nodepool)
		return
	}

	dyn.AutoscalerConfig = nil

	update := tracker.Result.Update()
	update.Kubernetes(action.State.K8S)
	update.Commit()
}
