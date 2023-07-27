package utils

import (
	"github.com/berops/claudie/proto/pb"
	"google.golang.org/protobuf/proto"
)

// ClusterView contains the per-cluster view on a given config.
// No mutex is needed when processing concurrently as long as each cluster only
// works with related values.
type ClusterView struct {
	// CurrentClusters are the individual clusters defined in the kubernetes section of the config of the current state.
	CurrentClusters map[string]*pb.K8Scluster
	// DesiredClusters are the individual clusters defined in the kubernetes section of the config of the desired state.
	DesiredClusters map[string]*pb.K8Scluster

	// Loadbalancers are the loadbalancers attach to a given kubernetes cluster in the current state.
	Loadbalancers map[string][]*pb.LBcluster
	// DesiredLoadbalancers are the loadbalancers attach to a given kubernetes cluster in the desired state.
	DesiredLoadbalancers map[string][]*pb.LBcluster

	// DeletedLoadbalancers are the loadbalancers that will be deleted (present in the current state but missing in the desired state)
	DeletedLoadbalancers map[string][]*pb.LBcluster

	// ClusterWorkflows is additional information per-cluster workflow (current stage of execution, if any error occurred etc..)
	ClusterWorkflows map[string]*pb.Workflow
}

func NewClusterView(config *pb.Config) *ClusterView {
	var (
		clusterWorkflows     = make(map[string]*pb.Workflow)
		clusters             = make(map[string]*pb.K8Scluster)
		desiredClusters      = make(map[string]*pb.K8Scluster)
		loadbalancers        = make(map[string][]*pb.LBcluster)
		desiredLoadbalancers = make(map[string][]*pb.LBcluster)
		deletedLoadbalancers = make(map[string][]*pb.LBcluster)
	)

	for _, current := range config.CurrentState.Clusters {
		clusters[current.ClusterInfo.Name] = current

		// store the cluster name with default workflow state.
		clusterWorkflows[current.ClusterInfo.Name] = &pb.Workflow{
			Stage:  pb.Workflow_NONE,
			Status: pb.Workflow_IN_PROGRESS,
		}
	}

	for _, desired := range config.DesiredState.Clusters {
		desiredClusters[desired.ClusterInfo.Name] = desired

		// store the cluster name with default workflow state.
		clusterWorkflows[desired.ClusterInfo.Name] = &pb.Workflow{
			Stage:  pb.Workflow_NONE,
			Status: pb.Workflow_IN_PROGRESS,
		}
	}

	for _, current := range config.CurrentState.LoadBalancerClusters {
		loadbalancers[current.TargetedK8S] = append(loadbalancers[current.TargetedK8S], current)
	}

	for _, desired := range config.DesiredState.LoadBalancerClusters {
		desiredLoadbalancers[desired.TargetedK8S] = append(desiredLoadbalancers[desired.TargetedK8S], desired)
	}

Lb:
	for _, current := range config.CurrentState.LoadBalancerClusters {
		for _, desired := range config.DesiredState.LoadBalancerClusters {
			if desired.ClusterInfo.Name == current.ClusterInfo.Name && desired.ClusterInfo.Hash == current.ClusterInfo.Hash {
				continue Lb
			}
		}
		deletedLoadbalancers[current.TargetedK8S] = append(deletedLoadbalancers[current.TargetedK8S], proto.Clone(current).(*pb.LBcluster))
	}

	return &ClusterView{
		CurrentClusters:      clusters,
		DesiredClusters:      desiredClusters,
		Loadbalancers:        loadbalancers,
		DesiredLoadbalancers: desiredLoadbalancers,
		DeletedLoadbalancers: deletedLoadbalancers,
		ClusterWorkflows:     clusterWorkflows,
	}
}

func mergeK8sClusters(old []*pb.K8Scluster, changed map[string]*pb.K8Scluster) []*pb.K8Scluster {
	// update existing clusters.
	for i, cluster := range old {
		cluster, ok := changed[cluster.ClusterInfo.Name]
		if !ok {
			old[i] = nil
			continue
		}
		old[i] = cluster
		delete(changed, cluster.ClusterInfo.Name)
	}

	// append new clusters, if any
	for _, cluster := range changed {
		old = append(old, cluster)
	}

	// remove unused
	for i := 0; i < len(old); {
		if old[i] != nil {
			i++
			continue
		}
		copy(old[i:], old[i+1:])
		old[len(old)-1] = nil
		old = old[:len(old)-1]
	}

	return old
}

func mergeLbClusters(old []*pb.LBcluster, changed map[string][]*pb.LBcluster) []*pb.LBcluster {
	// update existing clusters.
	for i, cluster := range old {
		updated, ok := changed[cluster.TargetedK8S]
		if !ok {
			old[i] = nil
			continue
		}
		present := GetLBClusterByName(cluster.ClusterInfo.Name, updated)
		if present < 0 {
			old[i] = nil
			continue
		}

		old[i] = updated[present]

		copy(updated[present:], updated[present+1:])
		updated[len(updated)-1] = nil
		changed[cluster.TargetedK8S] = changed[cluster.TargetedK8S][:len(updated)-1]

		if len(changed[cluster.TargetedK8S]) == 0 {
			delete(changed, cluster.TargetedK8S)
		}
	}

	// append new clusters, if any
	for _, clusters := range changed {
		old = append(old, clusters...)
	}

	// remove unused
	for i := 0; i < len(old); {
		if old[i] != nil {
			i++
			continue
		}
		copy(old[i:], old[i+1:])
		old[len(old)-1] = nil
		old = old[:len(old)-1]
	}

	return old
}

// MergeChanges propagates the changes made back to the config.
func (view *ClusterView) MergeChanges(config *pb.Config) {
	config.State = view.ClusterWorkflows

	config.CurrentState.Clusters = mergeK8sClusters(config.CurrentState.Clusters, view.CurrentClusters)
	config.DesiredState.Clusters = mergeK8sClusters(config.DesiredState.Clusters, view.DesiredClusters)

	config.CurrentState.LoadBalancerClusters = mergeLbClusters(config.CurrentState.LoadBalancerClusters, view.Loadbalancers)
	config.DesiredState.LoadBalancerClusters = mergeLbClusters(config.DesiredState.LoadBalancerClusters, view.DesiredLoadbalancers)
}

// AllClusters returns a slice of cluster all cluster names, from both the current state and desired state.
// This is useful to be abe to distinguish which clusters were deleted and which were not.
func (view *ClusterView) AllClusters() []string {
	clusters := make(map[string]struct{})

	for _, current := range view.CurrentClusters {
		clusters[current.ClusterInfo.Name] = struct{}{}
	}

	for _, desired := range view.DesiredClusters {
		clusters[desired.ClusterInfo.Name] = struct{}{}
	}

	c := make([]string, 0, len(clusters))
	for k := range clusters {
		c = append(c, k)
	}

	return c
}

func (view *ClusterView) SetWorkflowError(clusterName string, err error) {
	view.ClusterWorkflows[clusterName].Status = pb.Workflow_ERROR
	view.ClusterWorkflows[clusterName].Description = err.Error()
}

func (view *ClusterView) SetWorkflowDone(clusterName string) {
	view.ClusterWorkflows[clusterName].Status = pb.Workflow_DONE
	view.ClusterWorkflows[clusterName].Stage = pb.Workflow_NONE
	view.ClusterWorkflows[clusterName].Description = ""
}

func (view *ClusterView) UpdateCurrentState(clusterName string, c *pb.K8Scluster, lbs []*pb.LBcluster) {
	view.CurrentClusters[clusterName] = c
	view.Loadbalancers[clusterName] = lbs
}

func (view *ClusterView) UpdateDesiredState(clusterName string, c *pb.K8Scluster, lbs []*pb.LBcluster) {
	view.DesiredClusters[clusterName] = c
	view.DesiredLoadbalancers[clusterName] = lbs
}

func (view *ClusterView) RemoveCurrentState(clusterName string) {
	delete(view.CurrentClusters, clusterName)
	delete(view.Loadbalancers, clusterName)
}
