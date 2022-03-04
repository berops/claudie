package loadbalancer

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/clusterBuilder"
)

type LBcluster struct {
	DesiredLB   *pb.LBcluster
	CurrentLB   *pb.LBcluster
	ProjectName string
}

func (l LBcluster) Build() error {
	var currentInfo *pb.ClusterInfo
	var currentProvider *pb.Provider
	// check if current cluster was defined, to avoid access of unrefferenced memory
	if l.CurrentLB != nil {
		currentInfo = l.CurrentLB.ClusterInfo
		currentProvider = l.CurrentLB.Dns.Provider
	}
	cl := clusterBuilder.ClusterBuilder{
		DesiredInfo: l.DesiredLB.ClusterInfo,
		CurrentInfo: currentInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB}

	err := cl.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the LB cluster %s : %v", l.DesiredLB.ClusterInfo.Name, err)
	}
	nodeIPs := getNodeIPs(l.DesiredLB.ClusterInfo.NodePools)
	dns := DNS{
		ClusterName:     l.DesiredLB.ClusterInfo.Name,
		ClusterHash:     l.DesiredLB.ClusterInfo.Hash,
		DNSZone:         l.DesiredLB.Dns.DnsZone,
		NodeIPs:         nodeIPs,
		Project:         l.DesiredLB.Dns.Project,
		CurrentProvider: currentProvider,
		DesiredProvider: l.DesiredLB.Dns.Provider,
		Hostname:        l.DesiredLB.Dns.Hostname,
		ProjectName:     l.ProjectName,
	}
	endpoint, err := dns.CreateDNSrecords()
	if err != nil {
		return fmt.Errorf("error while creating the DNS for %s : %v", l.DesiredLB.ClusterInfo.Name, err)
	}
	l.DesiredLB.Dns.Endpoint = endpoint
	return nil
}

func (l LBcluster) Destroy() error {
	cluster := clusterBuilder.ClusterBuilder{
		//DesiredInfo: , //desired state is not used in DestroyNodepools
		CurrentInfo: l.CurrentLB.ClusterInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB}
	nodeIPs := getNodeIPs(l.CurrentLB.ClusterInfo.NodePools)
	dns := DNS{
		ClusterName:     l.CurrentLB.ClusterInfo.Name,
		ClusterHash:     l.CurrentLB.ClusterInfo.Hash,
		DNSZone:         l.CurrentLB.Dns.DnsZone,
		NodeIPs:         nodeIPs,
		Project:         l.CurrentLB.Dns.Project,
		CurrentProvider: l.CurrentLB.Dns.Provider,
		Hostname:        l.CurrentLB.Dns.Hostname,
		ProjectName:     l.ProjectName,
	}

	err := cluster.DestroyNodepools()
	if err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %v", l.CurrentLB.ClusterInfo.Name, err)
	}
	err = dns.DestroyDNSrecords()
	if err != nil {
		return fmt.Errorf("error while destroying the DNS records %s : %v", l.CurrentLB.ClusterInfo.Name, err)
	}
	return nil
}

func getNodeIPs(nodepools []*pb.NodePool) []string {
	var ips []string
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			ips = append(ips, node.Public)
		}
	}
	return ips
}
