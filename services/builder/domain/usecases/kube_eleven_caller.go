package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
)

// reconcileK8sCluster reconciles desired k8s cluster via kube-eleven.
func (u *Usecases) reconcileK8sCluster(ctx *builder.Context) error {
	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())

	// Set workflow state.
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, fmt.Sprintf("%s building kubernetes cluster", description))

	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(ctx.DesiredLoadbalancers); ep != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	logger.Info().Msgf("Calling BuildCluster on Kube-eleven")
	res, err := u.KubeEleven.BuildCluster(ctx, lbApiEndpoint, u.KubeEleven.GetClient())
	if err != nil {
		return fmt.Errorf("error while building kubernetes cluster %s project %s : %w", ctx.Id(), ctx.ProjectName, err)
	}
	logger.Info().Msgf("BuildCluster on Kube-eleven finished successfully")

	// Update desired state with returned data.
	ctx.DesiredCluster = res.Desired
	// Set description to original string.
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, description)
	return nil
}

func (u *Usecases) destroyK8sCluster(ctx *builder.Context) error {
	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())

	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, fmt.Sprintf("%s destroying kubernetes cluster", description))

	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(ctx.CurrentLoadbalancers); ep != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	logger.Info().Msgf("Calling DestroyCluster on Kube-eleven")
	res, err := u.KubeEleven.DestroyCluster(ctx, lbApiEndpoint, u.KubeEleven.GetClient())
	if err != nil {
		return fmt.Errorf("error while destroying kubernetes cluster %s project %s: %w", ctx.Id(), ctx.ProjectName, err)
	}

	logger.Info().Msgf("DestroyCluster on Kube-eleven finished successfully")

	ctx.CurrentCluster = res.Current

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBE_ELEVEN, description)
	return nil
}
