package usecases

import (
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

// removeClaudieUtilities removes previously installed claudie utilities.
func (u *Usecases) removeClaudieUtilities(ctx *utils.BuilderContext) error {
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
func (u *Usecases) configureInfrastructure(ctx *utils.BuilderContext) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())
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
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}

// callUpdateAPIEndpoint updates k8s API endpoint via ansibler.
func (u *Usecases) callUpdateAPIEndpoint(ctx *utils.BuilderContext, apiNodePool string) error {
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, fmt.Sprintf("%s changing api endpoint to a new control plane node", description))

	resp, err := u.Ansibler.UpdateAPIEndpoint(ctx, apiNodePool, u.Ansibler.GetClient())
	if err != nil {
		return err
	}

	ctx.CurrentCluster = resp.Current
	u.updateTaskWithDescription(ctx, spec.Workflow_ANSIBLER, description)
	return nil
}
