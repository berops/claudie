package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
)

// removeClaudieUtilities removes previously installed claudie utilities.
func (u *Usecases) removeClaudieUtilities(ctx *builder.Context) error {
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s removing claudie installed utilities", description))
	if err := u.Ansibler.RemoveClaudieUtilities(ctx, u.Ansibler.GetClient()); err != nil {
		return err
	}
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}

func (u *Usecases) updateProxyEnvsOnNodes(ctx *builder.Context) error {
	if ctx.ProxyEnvs.GetOp() == spec.ProxyOp_NONE {
		return nil
	}
	ctx.PopulateProxy()

	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())
	description := ctx.Workflow.Description

	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s updating proxy envs on nodes", description))
	logger.Info().Msgf("Calling UpdateProxyEnvsOnNodes on Ansibler")
	if err := u.Ansibler.UpdateProxyEnvsOnNodes(ctx, u.Ansibler.GetClient()); err != nil {
		return err
	}
	logger.Info().Msgf("UpdateProxyEnvsOnNodes on Ansibler finished successfully")
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}

func (u *Usecases) updateProxyEnvsInK8sServices(ctx *builder.Context) error {
	if ctx.ProxyEnvs.GetOp() == spec.ProxyOp_NONE {
		return nil
	}
	ctx.PopulateProxy()

	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())
	description := ctx.Workflow.Description

	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s updating proxy envs for k8s services", description))
	logger.Info().Msgf("Calling UpdateNoProxyEnvsInKubernetes on Ansibler")
	if err := u.Ansibler.UpdateProxyEnvsK8SServices(ctx, u.Ansibler.GetClient()); err != nil {
		return err
	}
	logger.Info().Msgf("UpdateNoProxyEnvsInKubernetes on Ansibler finished successfully")
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)

	return nil
}

// configureInfrastructure configures infrastructure via ansibler.
func (u *Usecases) configureInfrastructure(ctx *builder.Context) error {
	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())
	ansClient := u.Ansibler.GetClient()
	description := ctx.Workflow.Description

	// Update envs mainly for downloading packages.
	if err := u.updateProxyEnvsOnNodes(ctx); err != nil {
		return err
	}

	// Install VPN.
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s intalling VPN", description))

	logger.Info().Msgf("Calling InstallVPN on Ansibler")
	installRes, err := u.Ansibler.InstallVPN(ctx, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("InstallVPN on Ansibler finished successfully")

	ctx.DesiredCluster = installRes.Desired
	ctx.DesiredLoadbalancers = installRes.DesiredLbs

	// New nodes will now have private ips assigned, update proxy envs.
	if err := u.updateProxyEnvsOnNodes(ctx); err != nil {
		return err
	}

	// Install node requirements.
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s installing node requirements", description))

	logger.Info().Msgf("Calling InstallNodeRequirements on Ansibler")
	installRes, err = u.Ansibler.InstallNodeRequirements(ctx, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("InstallNodeRequirements on Ansibler finished successfully")

	ctx.DesiredCluster = installRes.Desired
	ctx.DesiredLoadbalancers = installRes.DesiredLbs

	// Set up Loadbalancers.
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s setting up Loadbalancers", description))

	logger.Info().Msgf("Calling SetUpLoadbalancers on Ansibler")
	setUpRes, err := u.Ansibler.SetUpLoadbalancers(ctx, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("SetUpLoadbalancers on Ansibler finished successfully")

	ctx.DesiredCluster = setUpRes.Desired
	ctx.DesiredLoadbalancers = setUpRes.DesiredLbs

	// propagate the proxy changes to k8s services.
	if err := u.updateProxyEnvsInK8sServices(ctx); err != nil {
		return err
	}

	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}

func (u *Usecases) determineApiEndpointChange(ctx *builder.Context, cid, did string, stt spec.ApiEndpointChangeState) error {
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s determining if API endpoint of the cluster should change based on the changes to the loadbalancers infrastructure", description))

	resp, err := u.Ansibler.DetermineApiEndpointChange(ctx, cid, did, stt, u.Ansibler.GetClient())
	if err != nil {
		return err
	}

	ctx.CurrentCluster = resp.Current
	ctx.CurrentLoadbalancers = resp.CurrentLbs
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}

// callUpdateAPIEndpoint updates k8s API endpoint via ansibler.
func (u *Usecases) callUpdateAPIEndpoint(ctx *builder.Context, nodepool, node string) error {
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s changing api endpoint to a new control plane node", description))

	resp, err := u.Ansibler.UpdateAPIEndpoint(ctx, nodepool, node, u.Ansibler.GetClient())
	if err != nil {
		return err
	}

	ctx.CurrentCluster = resp.Current
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}
