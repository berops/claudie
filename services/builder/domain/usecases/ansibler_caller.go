package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
)

// removeClaudieUtilities removes previously installed claudie utilities.
func (u *Usecases) removeClaudieUtilities(ctx *builder.Context) error {
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s removing claudie installed utilities", description))

	resp, err := u.Ansibler.RemoveClaudieUtilities(ctx, u.Ansibler.GetClient())
	if err != nil {
		return err
	}

	ctx.CurrentCluster = resp.Current
	ctx.CurrentLoadbalancers = resp.CurrentLbs

	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}

// configureInfrastructure configures infrastructure via ansibler.
func (u *Usecases) configureInfrastructure(ctx *builder.Context) error {
	logger := utils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())
	ansClient := u.Ansibler.GetClient()

	description := ctx.Workflow.Description

	// Tear down loadbalancers.
	apiEndpoint := ""
	if len(ctx.DeletedLoadBalancers) > 0 {
		u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s tearing down loadbalancers", description))

		logger.Info().Msgf("Calling TearDownLoadbalancers on Ansibler")
		teardownRes, err := u.Ansibler.TeardownLoadBalancers(ctx, ansClient)
		if err != nil {
			return err
		}
		logger.Info().Msgf("TearDownLoadbalancers on Ansibler finished successfully")

		ctx.DesiredCluster = teardownRes.Desired
		ctx.DesiredLoadbalancers = teardownRes.DesiredLbs
		ctx.DeletedLoadBalancers = teardownRes.DeletedLbs
		apiEndpoint = teardownRes.PreviousAPIEndpoint
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
	setUpRes, err := u.Ansibler.SetUpLoadbalancers(ctx, apiEndpoint, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("SetUpLoadbalancers on Ansibler finished successfully")

	ctx.DesiredCluster = setUpRes.Desired
	ctx.CurrentLoadbalancers = setUpRes.CurrentLbs
	ctx.DesiredLoadbalancers = setUpRes.DesiredLbs

	// Update NO_PROXY and no_proxy variables
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s updating NO_PROXY and no_proxy env variables", description))
	logger.Info().Msgf("Calling UpdateNoProxyEnvs on Ansibler")
	resp, err := u.Ansibler.UpdateNoProxyEnvs(ctx, ansClient)
	if err != nil {
		return err
	}

	logger.Info().Msgf("UpdateNoProxyEnvs on Ansibler finished successfully")
	ctx.CurrentCluster = resp.Current
	ctx.DesiredCluster = resp.Desired

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
