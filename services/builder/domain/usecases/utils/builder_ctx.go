package utils

import (
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

// BuilderContext provides context for the Claudie workflow for a particular cluster.
type BuilderContext struct {
	// ProjectName for this cluster
	ProjectName string
	// CurrentCluster is the current state of the cluster
	// properties may change during processing.
	CurrentCluster *pb.K8Scluster
	// DesiredCluster is the desired state of the cluster
	// properties may change during processing.
	DesiredCluster *pb.K8Scluster

	// CurrentLoadbalancers are the current loadbalancers of the cluster
	// properties may change during processing.
	CurrentLoadbalancers []*pb.LBcluster
	// DesiredLoadbalancers are the current loadbalancers of the cluster
	// properties may change during processing.
	DesiredLoadbalancers []*pb.LBcluster

	// DeletedLoadBalancers are the deleted loadbalancers for the cluster.
	DeletedLoadBalancers []*pb.LBcluster

	// Workflow is the current state of processing of the cluster.
	Workflow *pb.Workflow
}

// GetClusterName returns name of the k8s cluster for a given builder context.
func (ctx *BuilderContext) GetClusterName() string {
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
func (ctx *BuilderContext) GetClusterID() string {
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

// SplitCurrentCtx splits the builder current state context into two such that the
// former contains only the K8s context and the latter LB context.
func (ctx *BuilderContext) SplitCurrentCtx() (*BuilderContext, *BuilderContext) {
	k8sContext := &BuilderContext{
		ProjectName:          ctx.ProjectName,
		CurrentCluster:       ctx.CurrentCluster,
		DesiredCluster:       nil,
		CurrentLoadbalancers: nil,
		DesiredLoadbalancers: nil,
		DeletedLoadBalancers: nil,
		Workflow:             ctx.Workflow,
	}

	lbContext := &BuilderContext{
		ProjectName:          ctx.ProjectName,
		CurrentCluster:       nil,
		DesiredCluster:       nil,
		CurrentLoadbalancers: ctx.CurrentLoadbalancers,
		DesiredLoadbalancers: nil,
		DeletedLoadBalancers: nil,
		Workflow:             ctx.Workflow,
	}

	return k8sContext, lbContext
}
