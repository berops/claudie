package kubernetes

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/cluster"
)

type Kubernetes struct {
	DesiredK8s  *pb.K8Scluster
	CurrentK8s  *pb.K8Scluster
	ProjectName string
}

func (k Kubernetes) Build() error {
	cluster := cluster.Cluster{
		DesiredInfo: k.DesiredK8s.ClusterInfo,
		CurrentInfo: k.CurrentK8s.ClusterInfo,
		ProjectName: k.ProjectName,
		ClusterType: pb.ClusterType_K8s}
	err := cluster.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the K8s cluster %s : %v", k.DesiredK8s.ClusterInfo.Name, err)
	}
	return nil
}

func (k Kubernetes) Destroy() error {
	cluster := cluster.Cluster{
		DesiredInfo: k.DesiredK8s.ClusterInfo,
		CurrentInfo: k.CurrentK8s.ClusterInfo,
		ProjectName: k.ProjectName,
		ClusterType: pb.ClusterType_K8s}
	err := cluster.DestroyNodepools()
	if err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %v", k.DesiredK8s.ClusterInfo.Name, err)
	}
	return nil
}
