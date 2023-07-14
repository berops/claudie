package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	ansibler "github.com/berops/claudie/services/ansibler/client"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	kuber "github.com/berops/claudie/services/kuber/client"
)

// callAnsibler passes config to ansibler to set up VPN
func (u *Usecases) configureInfrastructure(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_ANSIBLER
	ctx.Workflow.Description = fmt.Sprintf("%s tearing down loadbalancers", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := cutils.GrpcDialWithRetryAndBackoff("ansibler", envs.AnsiblerURL)
	if err != nil {
		return err
	}
	defer cutils.CloseClientConnection(cc)

	c := pb.NewAnsiblerServiceClient(cc)

	// Call TearDownLoadbalancers only when its needed.
	apiEndpoint := ""
	if len(ctx.DeletedLoadBalancers) > 0 {
		logger.Info().Msgf("Calling TearDownLoadbalancers on Ansibler")
		teardownRes, err := ansibler.TeardownLoadBalancers(c, &pb.TeardownLBRequest{
			Desired:     ctx.DesiredCluster,
			DesiredLbs:  ctx.DesiredLoadbalancers,
			DeletedLbs:  ctx.DeletedLoadBalancers,
			ProjectName: ctx.ProjectName,
		})
		if err != nil {
			return err
		}
		logger.Info().Msgf("TearDownLoadbalancers on Ansibler finished successfully")

		ctx.DesiredCluster = teardownRes.Desired
		ctx.DesiredLoadbalancers = teardownRes.DesiredLbs
		ctx.DeletedLoadBalancers = teardownRes.DeletedLbs
		apiEndpoint = teardownRes.PreviousAPIEndpoint
	}

	ctx.Workflow.Description = fmt.Sprintf("%s installing VPN", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling InstallVPN on Ansibler")
	installRes, err := ansibler.InstallVPN(c, &pb.InstallRequest{
		Desired:     ctx.DesiredCluster,
		DesiredLbs:  ctx.DesiredLoadbalancers,
		ProjectName: ctx.ProjectName,
	})
	if err != nil {
		return err
	}
	logger.Info().Msgf("InstallVPN on Ansibler finished successfully")

	ctx.DesiredCluster = installRes.Desired
	ctx.DesiredLoadbalancers = installRes.DesiredLbs

	ctx.Workflow.Description = fmt.Sprintf("%s installing node requirements", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling InstallNodeRequirements on Ansibler")
	installRes, err = ansibler.InstallNodeRequirements(c, &pb.InstallRequest{
		Desired:     ctx.DesiredCluster,
		DesiredLbs:  ctx.DesiredLoadbalancers,
		ProjectName: ctx.ProjectName,
	})
	if err != nil {
		return err
	}
	logger.Info().Msgf("InstallNodeRequirements on Ansibler finished successfully")

	ctx.DesiredCluster = installRes.Desired
	ctx.DesiredLoadbalancers = installRes.DesiredLbs

	ctx.Workflow.Description = fmt.Sprintf("%s setting up loadbalancers", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling SetUpLoadbalancers on Ansibler")
	setUpRes, err := ansibler.SetUpLoadbalancers(c, &pb.SetUpLBRequest{
		Desired:             ctx.DesiredCluster,
		CurrentLbs:          ctx.CurrentLoadbalancers,
		DesiredLbs:          ctx.DesiredLoadbalancers,
		PreviousAPIEndpoint: apiEndpoint,
		ProjectName:         ctx.ProjectName,
		FirstRun:            ctx.CurrentCluster == nil,
	})
	if err != nil {
		return err
	}
	logger.Info().Msgf("SetUpLoadbalancers on Ansibler finished successfully")

	ctx.DesiredCluster = setUpRes.Desired
	ctx.CurrentLoadbalancers = setUpRes.CurrentLbs
	ctx.DesiredLoadbalancers = setUpRes.DesiredLbs

	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	return nil
}

func (u *Usecases) callPatchClusterInfoConfigMap(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_KUBER

	cc, err := cutils.GrpcDialWithRetryAndBackoff("kuber", envs.KuberURL)
	if err != nil {
		return err
	}
	defer cutils.CloseClientConnection(cc)

	c := pb.NewKuberServiceClient(cc)

	ctx.Workflow.Description = fmt.Sprintf("%s patching cluster info config map", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling PatchClusterInfoConfigMap on kuber for cluster")
	if err := kuber.PatchClusterInfoConfigMap(c, &pb.PatchClusterInfoConfigMapRequest{DesiredCluster: ctx.DesiredCluster}); err != nil {
		return err
	}
	logger.Info().Msg("PatchClusterInfoConfigMap on Kuber for cluster finished successfully")

	ctx.Workflow.Description = description
	return u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient)
}

func (u *Usecases) callUpdateAPIEndpoint(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_ANSIBLER
	ctx.Workflow.Description = fmt.Sprintf("%s changing api endpoint to a new control plane node", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := cutils.GrpcDialWithRetryAndBackoff("ansibler", envs.AnsiblerURL)
	if err != nil {
		return err
	}
	defer cutils.CloseClientConnection(cc)

	c := pb.NewAnsiblerServiceClient(cc)

	resp, err := ansibler.UpdateAPIEndpoint(c, &pb.UpdateAPIEndpointRequest{
		Current:     ctx.CurrentCluster,
		Desired:     ctx.DesiredCluster,
		ProjectName: ctx.ProjectName,
	})

	if err != nil {
		return err
	}

	ctx.CurrentCluster = resp.Current
	ctx.DesiredCluster = resp.Desired

	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	return nil
}
