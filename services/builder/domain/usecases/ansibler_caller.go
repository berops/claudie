package usecases

import (
	"context"

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

	tasks := []Task{
		{
			// update environment variables for downloading packages.
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
		{
			do:          u.installTeeOverride,
			stage:       spec.Workflow_ANSIBLER,
			description: "installing tee binary override",
		},
	}

	return u.processTasks(ctx, work, logger, tasks)
}

func (u *Usecases) determineApiEndpointChange(cid, did string, stt spec.ApiEndpointChangeState) Task {
	return Task{
		do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
			resp, err := u.Ansibler.DetermineApiEndpointChange(work, cid, did, stt, u.Ansibler.GetClient())
			if err != nil {
				return err
			}
			work.CurrentCluster = resp.Current
			work.CurrentLoadbalancers = resp.CurrentLbs
			return nil
		},
		stage:       spec.Workflow_ANSIBLER,
		description: "determining if API endpoint of the kubernetes cluster should change based on the new loadbalancer infrastructure",
	}
}

func (u *Usecases) updateControlPlaneApiEndpoint(nodepool, node string) Task {
	return Task{
		do: func(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
			resp, err := u.Ansibler.UpdateAPIEndpoint(work, nodepool, node, u.Ansibler.GetClient())
			if err != nil {
				return err
			}
			work.CurrentCluster = resp.Current
			return nil
		},
		stage:       spec.Workflow_ANSIBLER,
		description: "changing api endpoint to a new control plane node",
	}
}

func (u *Usecases) installTeeOverride(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	if err := u.Ansibler.InstallTeeOverride(work, u.Ansibler.GetClient()); err != nil {
		return err
	}
	return nil
}
