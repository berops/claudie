package builder

import (
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
)

// Context provides context for the Claudie workflow for a particular cluster.
type Context struct {
	// ProjectName name of the config from which the cluster is.
	ProjectName string
	// TaskId from which this process was spawned from.
	TaskId string
	// CurrentCluster is the current state of the cluster
	// properties may change during processing.
	CurrentCluster *spec.K8Scluster
	// DesiredCluster is the desired state of the cluster
	// properties may change during processing.
	DesiredCluster *spec.K8Scluster

	// CurrentLoadbalancers are the current loadbalancers of the cluster
	// properties may change during processing.
	CurrentLoadbalancers []*spec.LBcluster
	// DesiredLoadbalancers are the current loadbalancers of the cluster
	// properties may change during processing.
	DesiredLoadbalancers []*spec.LBcluster

	// DeletedLoadBalancers are the deleted loadbalancers for the cluster.
	DeletedLoadBalancers []*spec.LBcluster

	// Workflow is the current state of processing of the cluster.
	Workflow *spec.Workflow

	// ProxyEnvs holds information about a need to update proxy envs, proxy endpoint, and no proxy list.
	ProxyEnvs *spec.ProxyEnvs
}

// GetClusterName returns name of the k8s cluster for a given builder context.
func (ctx *Context) GetClusterName() string {
	if ctx.DesiredCluster != nil {
		return ctx.DesiredCluster.ClusterInfo.Name
	}
	if ctx.CurrentCluster != nil {
		return ctx.CurrentCluster.ClusterInfo.Name
	}

	// try to get the cluster name from the lbs if present
	if len(ctx.DesiredLoadbalancers) != 0 {
		return ctx.DesiredLoadbalancers[0].TargetedK8S
	}

	if len(ctx.CurrentLoadbalancers) != 0 {
		return ctx.CurrentLoadbalancers[0].TargetedK8S
	}

	if len(ctx.DeletedLoadBalancers) != 0 {
		return ctx.DeletedLoadBalancers[0].TargetedK8S
	}

	return ""
}

// GetClusterID returns ID of the k8s cluster for a given builder context.
func (ctx *Context) GetClusterID() string {
	if ctx.DesiredCluster != nil {
		return utils.GetClusterID(ctx.DesiredCluster.ClusterInfo)
	}
	if ctx.CurrentCluster != nil {
		return utils.GetClusterID(ctx.CurrentCluster.ClusterInfo)
	}

	// try to get the cluster name from the lbs if present
	if len(ctx.DesiredLoadbalancers) != 0 {
		return ctx.DesiredLoadbalancers[0].TargetedK8S
	}

	if len(ctx.CurrentLoadbalancers) != 0 {
		return ctx.CurrentLoadbalancers[0].TargetedK8S
	}

	if len(ctx.DeletedLoadBalancers) != 0 {
		return ctx.DeletedLoadBalancers[0].TargetedK8S
	}

	return ""
}

func (ctx *Context) DetermineProxyUpdate() bool {
	updateProxyEnvsFlag := false
	defaultMode := "default"
	offMode := "off"

	firstRun := ctx.CurrentCluster == nil || ctx.CurrentCluster.Kubeconfig == ""
	if firstRun {
		// The cluster wasn't build yet because currentState is nil
		// but we have to check if the proxy is turned on or in a default mode with Hetzner node in the desired state.
		desiredProxySettings := ctx.DesiredCluster.InstallationProxy

		if desiredProxySettings.Mode == defaultMode && HasHetznerNode(ctx.DesiredCluster.ClusterInfo) {
			updateProxyEnvsFlag = true
		} else if desiredProxySettings.Mode == "on" {
			updateProxyEnvsFlag = true
		}
	} else if ctx.CurrentCluster != nil && ctx.DesiredCluster != nil {
		updateProxyEnvsFlag = true

		hetznerInDesired := HasHetznerNode(ctx.DesiredCluster.ClusterInfo)
		hetznerInCurrent := HasHetznerNode(ctx.CurrentCluster.ClusterInfo)

		currProxySettings := ctx.CurrentCluster.InstallationProxy
		desiredProxySettings := ctx.DesiredCluster.InstallationProxy

		// The following use cases represents scenarios when the proxy envs have to be updated:
		// The proxy mode is on in both current and the desired state.
		// The proxy mode is default in both current and the desired state. Both states have at least one Hetzner node.
		// The proxy mode is default in both current and the desired state. The desired state has at least one Hetzner node. The current state doesn't have any Hetzner nodes.
		// The proxy mode is default in both current and the desired state. The desired state doesn't have any Hetzner nodes. The current state have at least one Hetzner node.
		// The proxy mode is default in both current and the desired state. Both states don't have any Hetzner nodes.
		// The proxy mode is default in the current state. The current state doesn't have any Hetzner nodes. The proxy is turned on in the desired state.
		// The proxy mode is default in the current state. The current state has at least one Hetzner node. The proxy is turned off in the desired state.
		// The proxy mode is default in the current state. The current state has at least one Hetzner node. The proxy is turned on in the desired state.
		// The proxy mode is off in the current state. The proxy modes is on in the desired state.
		// The proxy mode is on in the current state. The desired state has at least one Hetzner node and the proxy mode is default.
		// The proxy mode is on in the current state. The desired state doesn't have any Hetzner nodes and the proxy mode is default.
		// The proxy mode is off in the current state. The proxy mode is on in the desired state.
		// The proxy mode is off in the current state. The desired state has at least one Hetzner node and the proxy mode is default.

		// The following use cases represents scenarios when the proxy envs don't have to be updated:
		// The proxy mode is off in both current and the desired state.
		// The proxy mode is default in both current and the desired state. Both states don't have any Hetzner nodes.
		// The proxy mode is default in the current state. The current state doesn't have any Hetzner nodes. The proxy is turned off in the desired state.
		// The proxy mode is off in the current state. The desired state doesn't have any Hetzner nodes and the proxy mode is default.
		switch {
		case currProxySettings.Mode == offMode && desiredProxySettings.Mode == offMode:
			// The proxy is and was turned off in both cases.
			updateProxyEnvsFlag = false
		case currProxySettings.Mode == defaultMode && desiredProxySettings.Mode == defaultMode && !hetznerInCurrent && !hetznerInDesired:
			// The proxy is in default mode without Hetzner node in both cases.
			updateProxyEnvsFlag = false
		case currProxySettings.Mode == defaultMode && !hetznerInCurrent && desiredProxySettings.Mode == offMode:
			// The proxy was in default mode without Hetzner node and is turned off.
			updateProxyEnvsFlag = false
		case currProxySettings.Mode == offMode && desiredProxySettings.Mode == defaultMode && !hetznerInDesired:
			// The proxy was in off mode. Now it is in default mode without Hetzner node.
			updateProxyEnvsFlag = false
		}
	}

	return updateProxyEnvsFlag
}
