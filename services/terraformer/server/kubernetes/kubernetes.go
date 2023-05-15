package kubernetes

import (
	"fmt"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/clusterBuilder"
)

type K8Scluster struct {
	ProjectName string

	DesiredState *pb.K8Scluster
	CurrentState *pb.K8Scluster

	// AttachedLBClusters are the LB clusters that are
	// attached to this K8s cluster.
	AttachedLBClusters []*pb.LBcluster
}

func (k K8Scluster) Id() string {
	state := k.DesiredState
	if state == nil {
		state = k.CurrentState
	}

	return fmt.Sprintf("%s-%s", state.ClusterInfo.Name, state.ClusterInfo.Hash)
}

func (k K8Scluster) Build() error {
	var currentClusterInfo *pb.ClusterInfo
	// Check if current cluster was defined, to avoid access of unreferenced memory
	if k.CurrentState != nil {
		currentClusterInfo = k.CurrentState.ClusterInfo
	}

	cluster := clusterBuilder.ClusterBuilder{
		DesiredClusterInfo: k.DesiredState.ClusterInfo,
		CurrentClusterInfo: currentClusterInfo,

		ProjectName: k.ProjectName,
		ClusterType: pb.ClusterType_K8s,
		Metadata: map[string]any{
			"attachedLBClusters": k.AttachedLBClusters,
		},
	}

	err := cluster.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the K8s cluster %s : %w", k.DesiredState.ClusterInfo.Name, err)
	}

	return nil
}

func (k K8Scluster) Destroy() error {
	cluster := clusterBuilder.ClusterBuilder{
		CurrentClusterInfo: k.CurrentState.ClusterInfo,

		ProjectName: k.ProjectName,
		ClusterType: pb.ClusterType_K8s,
	}

	err := cluster.DestroyNodepools()
	if err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %w", k.CurrentState.ClusterInfo.Name, err)
	}

	return nil
}
