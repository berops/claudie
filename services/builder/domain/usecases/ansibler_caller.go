package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
	"github.com/rs/zerolog"
)

// removeClaudieUtilities removes previously installed claudie utilities.
func (u *Usecases) removeClaudieUtilities(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	return u.Ansibler.RemoveClaudieUtilities(work, u.Ansibler.GetClient())
}

func (u *Usecases) updateProxyEnvsOnNodes(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	if work.ProxyEnvs.GetOp() == spec.ProxyOp_NONE {
		return nil
	}
	work.PopulateProxy()
	return u.Ansibler.UpdateProxyEnvsOnNodes(work, u.Ansibler.GetClient())
}

func (u *Usecases) installVPN(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	installRes, err := u.Ansibler.InstallVPN(work, u.Ansibler.GetClient())
	if err != nil {
		return err
	}
	work.DesiredCluster = installRes.Desired
	work.DesiredLoadbalancers = installRes.DesiredLbs
	return nil
}

func (u *Usecases) installNodeRequirements(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	installRes, err := u.Ansibler.InstallNodeRequirements(work, u.Ansibler.GetClient())
	if err != nil {
		return err
	}
	work.DesiredCluster = installRes.Desired
	work.DesiredLoadbalancers = installRes.DesiredLbs
	return nil
}

func (u *Usecases) setupLoadBalancers(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	setUpRes, err := u.Ansibler.SetUpLoadbalancers(work, u.Ansibler.GetClient())
	if err != nil {
		return err
	}
	work.DesiredCluster = setUpRes.Desired
	work.DesiredLoadbalancers = setUpRes.DesiredLbs
	return nil
}

func (u *Usecases) updateProxyEnvsInK8sServices(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	if work.ProxyEnvs.GetOp() == spec.ProxyOp_NONE {
		return nil
	}
	work.PopulateProxy()
	return u.Ansibler.UpdateProxyEnvsK8SServices(work, u.Ansibler.GetClient())
}

// configureInfrastructure configures infrastructure via ansibler.
func (u *Usecases) configureInfrastructure(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	work.ProxyEnvs = &spec.ProxyEnvs{
		Op: builder.DetermineProxyOperation(work),
	}
	return u.processTasks(ctx, work, logger, []Task{
		{
			// update environemnt variables for donwloading packages.
			do:          u.updateProxyEnvsOnNodes,
			stage:       spec.Workflow_ANSIBLER,
			description: "updating proxy environment variables",
		},
		{
			do:          u.installVPN,
			stage:       spec.Workflow_ANSIBLER,
			description: "installing VPN",
		},
		{
			// all nodes will now have assigned private ips, re-update environment
			// variables with all of the IPs.
			do:          u.updateProxyEnvsOnNodes,
			stage:       spec.Workflow_ANSIBLER,
			description: "updating proxy environtment variables with new IPs",
		},
		{
			do:          u.installNodeRequirements,
			stage:       spec.Workflow_ANSIBLER,
			description: "installing node requirements",
		},
		{
			do:          u.setupLoadBalancers,
			stage:       spec.Workflow_ANSIBLER,
			description: "setting up loadbalancers",
		},
		{
			// propagate the proxy environment to necessary kuberentes services.
			do:          u.updateProxyEnvsInK8sServices,
			stage:       spec.Workflow_ANSIBLER,
			description: "updating proxy environment variables for kuberentes services",
		},
	})
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
