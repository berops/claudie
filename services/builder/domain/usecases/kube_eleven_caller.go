package usecases

import (
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

// reconcileK8sCluster reconciles desired k8s cluster via kube-eleven.
func (u *Usecases) reconcileK8sCluster(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	// Set workflow state.
	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_KUBE_ELEVEN
	ctx.Workflow.Description = fmt.Sprintf("%s building kubernetes cluster", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

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
	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	return nil
}
