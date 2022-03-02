package loadbalancer

import (
	"fmt"
	"path/filepath"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/cluster"
	"github.com/Berops/platform/services/terraformer/server/templates"
	"github.com/Berops/platform/services/terraformer/server/terraform"
)

type Loadbalancer struct {
	DesiredLB   *pb.LBcluster
	CurrentLB   *pb.LBcluster
	ProjectName string
}

func (l Loadbalancer) Build() error {
	cl := cluster.Cluster{
		DesiredInfo: l.DesiredLB.ClusterInfo,
		CurrentInfo: l.CurrentLB.ClusterInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB}
	err := cl.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the K8s cluster %s : %v", k.DesiredK8s.ClusterInfo.Name, err)
	}
	l.createDNS()
	return nil
}

func (l Loadbalancer) Destroy() error {
	cluster := cluster.Cluster{
		DesiredInfo: l.DesiredLB.ClusterInfo,
		CurrentInfo: l.CurrentLB.ClusterInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB}
	err := cluster.DestroyNodepools()
	if err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %v", k.DesiredK8s.ClusterInfo.Name, err)
	}
	return nil
}

func (l Loadbalancer) createDNS() {
	clusterID := fmt.Sprintf(l.DesiredLB.ClusterInfo.Name, "-", l.DesiredLB.ClusterInfo.Hash)
	clusterDir := filepath.Join(cluster.Output, clusterID)
	terraform := terraform.Terraform{Directory: clusterDir}
	templates := templates.Templates{Directory: clusterDir}

}
