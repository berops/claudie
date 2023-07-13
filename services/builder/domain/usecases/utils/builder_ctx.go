package utils

import (
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

type BuilderContext struct {
	ProjectName string
	// cluster is the current state of the cluster
	// properties may change during processing.
	CurrentCluster *pb.K8Scluster
	// desiredCluster is the desired state of the cluster
	// properties may change during processing.
	DesiredCluster *pb.K8Scluster

	// loadbalancers are the current loadbalancers of the cluster
	// properties may change during processing.
	CurrentLoadbalancers []*pb.LBcluster
	// desiredLoadbalancers are the current loadbalancers of the cluster
	// properties may change during processing.
	DesiredLoadbalancers []*pb.LBcluster

	// deletedLoadBalancers are the deleted loadbalancers for the cluster.
	DeletedLoadBalancers []*pb.LBcluster

	// Workflow is the current state of processing of the cluster.
	Workflow *pb.Workflow
}

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

// updateFromBuild takes the changes after a successful workflow of a given cluster
func UpdateFromBuild(ctx *BuilderContext, view *utils.ClusterView) {
	if ctx.CurrentCluster != nil {
		view.CurrentClusters[ctx.CurrentCluster.ClusterInfo.Name] = ctx.CurrentCluster
	}

	if ctx.DesiredCluster != nil {
		view.DesiredClusters[ctx.DesiredCluster.ClusterInfo.Name] = ctx.DesiredCluster
	}

	if ctx.Workflow != nil {
		view.ClusterWorkflows[ctx.GetClusterName()] = ctx.Workflow
	}

	for _, current := range ctx.CurrentLoadbalancers {
		for i := range view.Loadbalancers[current.TargetedK8S] {
			if view.Loadbalancers[current.TargetedK8S][i].ClusterInfo.Name == current.ClusterInfo.Name {
				view.Loadbalancers[current.TargetedK8S][i] = current
				break
			}
		}
	}

	for _, desired := range ctx.DesiredLoadbalancers {
		for i := range view.DesiredLoadbalancers[desired.TargetedK8S] {
			if view.DesiredLoadbalancers[desired.TargetedK8S][i].ClusterInfo.Name == desired.ClusterInfo.Name {
				view.DesiredLoadbalancers[desired.TargetedK8S][i] = desired
				break
			}
		}
	}

	for _, deleted := range ctx.DeletedLoadBalancers {
		for i := range view.DeletedLoadbalancers[deleted.TargetedK8S] {
			if view.DeletedLoadbalancers[deleted.TargetedK8S][i].ClusterInfo.Name == deleted.ClusterInfo.Name {
				view.DeletedLoadbalancers[deleted.TargetedK8S][i] = deleted
				break
			}
		}
	}
}
