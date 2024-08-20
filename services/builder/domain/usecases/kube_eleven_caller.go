package usecases

import (
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

// reconcileK8sCluster reconciles desired k8s cluster via kube-eleven.
func (u *Usecases) reconcileK8sCluster(ctx *utils.BuilderContext) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	// Set workflow state.
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, fmt.Sprintf("%s building kubernetes cluster", description))

	logger.Info().Msgf("Calling BuildCluster on Kube-eleven")
	res, err := u.KubeEleven.BuildCluster(ctx, u.KubeEleven.GetClient())
	if err != nil {
		return fmt.Errorf("error while building kubernetes cluster %s project %s : %w", ctx.GetClusterID(), ctx.ProjectName, err)
	}
	logger.Info().Msgf("BuildCluster on Kube-eleven finished successfully")

	// Update desired state with returned data.
	ctx.DesiredCluster = res.Desired
	ctx.DesiredLoadbalancers = res.DesiredLbs
	// Set description to original string.
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, description)
	return nil
}

func (u *Usecases) destroyK8sCluster(ctx *utils.BuilderContext) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, fmt.Sprintf("%s destroying kubernetes cluster", description))

	logger.Info().Msgf("Calling DestroyCluster on Kube-eleven")
	res, err := u.KubeEleven.DestroyCluster(ctx, u.KubeEleven.GetClient())
	if err != nil {
		return fmt.Errorf("error while destroying kubernetes cluster %s project %s: %w", ctx.GetClusterID(), ctx.ProjectName, err)
	}

	logger.Info().Msgf("DestroyCluster on Kube-eleven finished successfully")

	ctx.CurrentCluster = res.Current
	ctx.CurrentLoadbalancers = res.CurrentLbs

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, description)
	return nil
}
