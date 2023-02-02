package loadbalancer

import (
	"fmt"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/terraformer/server/clusterBuilder"
)

type LBcluster struct {
	DesiredLB   *pb.LBcluster
	CurrentLB   *pb.LBcluster
	ProjectName string
}

func (l LBcluster) Id() string {
	state := l.DesiredLB
	if state == nil {
		state = l.CurrentLB
	}

	return fmt.Sprintf("%s-%s", state.ClusterInfo.Name, state.ClusterInfo.Hash)
}

func (l LBcluster) Build() error {
	var currentInfo *pb.ClusterInfo
	var currentDNS *pb.DNS
	var currentNodeIPs []string

	// check if current cluster was defined, to avoid access of unrefferenced memory
	if l.CurrentLB != nil {
		currentInfo = l.CurrentLB.ClusterInfo
		currentDNS = l.CurrentLB.Dns
		currentNodeIPs = getNodeIPs(l.CurrentLB.ClusterInfo.NodePools)
	}

	cl := clusterBuilder.ClusterBuilder{
		DesiredInfo: l.DesiredLB.ClusterInfo,
		CurrentInfo: currentInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB,
		Metadata: map[string]any{
			"roles": l.DesiredLB.Roles,
		},
	}

	err := cl.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the LB cluster %s : %w", l.DesiredLB.ClusterInfo.Name, err)
	}

	nodeIPs := getNodeIPs(l.DesiredLB.ClusterInfo.NodePools)
	dns := DNS{
		ClusterName:    l.DesiredLB.ClusterInfo.Name,
		ClusterHash:    l.DesiredLB.ClusterInfo.Hash,
		CurrentNodeIPs: currentNodeIPs,
		DesiredNodeIPs: nodeIPs,
		CurrentDNS:     currentDNS,
		DesiredDNS:     l.DesiredLB.Dns,
		ProjectName:    l.ProjectName,
	}

	endpoint, err := dns.CreateDNSRecords()
	if err != nil {
		return fmt.Errorf("error while creating the DNS for %s : %w", l.DesiredLB.ClusterInfo.Name, err)
	}

	l.DesiredLB.Dns.Endpoint = endpoint

	return nil
}

func (l LBcluster) Destroy() error {
	group := errgroup.Group{}

	group.Go(func() error {
		cluster := clusterBuilder.ClusterBuilder{
			//DesiredInfo: desired state is not used in Destroy
			CurrentInfo: l.CurrentLB.ClusterInfo,
			ProjectName: l.ProjectName,
			ClusterType: pb.ClusterType_LB,
		}
		return cluster.DestroyNodepools()
	})

	group.Go(func() error {
		dns := DNS{
			ClusterName:    l.CurrentLB.ClusterInfo.Name,
			ClusterHash:    l.CurrentLB.ClusterInfo.Hash,
			CurrentNodeIPs: getNodeIPs(l.CurrentLB.ClusterInfo.NodePools),
			CurrentDNS:     l.CurrentLB.Dns,
			ProjectName:    l.ProjectName,
		}
		return dns.DestroyDNSRecords()
	})

	return group.Wait()
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
