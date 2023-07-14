package usecases

import (
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

// callAnsibler passes config to ansibler to set up VPN
func (u *Usecases) configureInfrastructure(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())
	ansClient := u.Ansibler.GetClient()
	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_ANSIBLER

	// Tear down loadbalancers.
	apiEndpoint := ""
	if len(ctx.DeletedLoadBalancers) > 0 {
		if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s tearing down loadbalancers", description), cboxClient); err != nil {
			return err
		}
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
	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s installing VPN", description), cboxClient); err != nil {
		return err
	}
	logger.Info().Msgf("Calling InstallVPN on Ansibler")
	installRes, err := u.Ansibler.InstallVPN(ctx, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("InstallVPN on Ansibler finished successfully")

	ctx.DesiredCluster = installRes.Desired
	ctx.DesiredLoadbalancers = installRes.DesiredLbs

	// Install node requirements.
	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s installing node requirements", description), cboxClient); err != nil {
		return err
	}
	logger.Info().Msgf("Calling InstallNodeRequirements on Ansibler")
	installRes, err = u.Ansibler.InstallNodeRequirements(ctx, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("InstallNodeRequirements on Ansibler finished successfully")

	ctx.DesiredCluster = installRes.Desired
	ctx.DesiredLoadbalancers = installRes.DesiredLbs

	// Set up Loadbalancers.
	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s setting up Loadbalancers", description), cboxClient); err != nil {
		return err
	}
	logger.Info().Msgf("Calling SetUpLoadbalancers on Ansibler")
	setUpRes, err := u.Ansibler.SetUpLoadbalancers(ctx, apiEndpoint, ansClient)
	if err != nil {
		return err
	}
	logger.Info().Msgf("SetUpLoadbalancers on Ansibler finished successfully")

	ctx.DesiredCluster = setUpRes.Desired
	ctx.CurrentLoadbalancers = setUpRes.CurrentLbs
	ctx.DesiredLoadbalancers = setUpRes.DesiredLbs
	if err := u.saveWorkflowDescription(ctx, description, cboxClient); err != nil {
		return err
	}
	return nil
}

// callUpdateAPIEndpoint updates k8s API endpoint via ansibler.
func (u *Usecases) callUpdateAPIEndpoint(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_ANSIBLER
	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s changing api endpoint to a new control plane node", description), cboxClient); err != nil {
		return err
	}

	resp, err := u.Ansibler.UpdateAPIEndpoint(ctx, u.Ansibler.GetClient())
	if err != nil {
		return err
	}

	ctx.CurrentCluster = resp.Current
	ctx.DesiredCluster = resp.Desired
	if err := u.saveWorkflowDescription(ctx, description, cboxClient); err != nil {
		return err
	}

	return nil
}
