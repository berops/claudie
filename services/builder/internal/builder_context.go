package builder

import (
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

	// Workflow is the current state of processing of the cluster.
	Workflow *spec.Workflow

	// ProxyEnvs holds information about a need to update proxy envs, proxy endpoint, and no proxy list.
	ProxyEnvs *spec.ProxyEnvs

	// Options that were set by the manager service for the task this context was created for.
	Options uint64
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

	return ""
}

// Id returns Id of the k8s cluster for a given builder context.
func (ctx *Context) Id() string {
	if ctx.DesiredCluster != nil {
		return ctx.DesiredCluster.ClusterInfo.Id()
	}
	if ctx.CurrentCluster != nil {
		return ctx.CurrentCluster.ClusterInfo.Id()
	}

	// try to get the cluster name from the lbs if present
	if len(ctx.DesiredLoadbalancers) != 0 {
		return ctx.DesiredLoadbalancers[0].TargetedK8S
	}

	if len(ctx.CurrentLoadbalancers) != 0 {
		return ctx.CurrentLoadbalancers[0].TargetedK8S
	}

	return ""
}

func (ctx *Context) PopulateProxy() {
	if ctx.ProxyEnvs == nil {
		return
	}
	if ctx.ProxyEnvs.GetOp() == spec.ProxyOp_OFF {
		ctx.ProxyEnvs.HttpProxyUrl, ctx.ProxyEnvs.NoProxyList = "", ""
	} else {
		ctx.ProxyEnvs.HttpProxyUrl, ctx.ProxyEnvs.NoProxyList = HttpProxyUrlAndNoProxyList(
			ctx.DesiredCluster,
			ctx.DesiredLoadbalancers,
			HasHetznerNode(ctx.DesiredCluster),
		)
	}
}
