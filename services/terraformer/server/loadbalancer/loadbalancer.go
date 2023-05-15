package loadbalancer

import (
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/clusterBuilder"
)

type LBcluster struct {
	DesiredState *pb.LBcluster
	CurrentState *pb.LBcluster

	ProjectName string
}

func (l LBcluster) Id() string {
	state := l.DesiredState
	if state == nil {
		state = l.CurrentState
	}

	return fmt.Sprintf("%s-%s", state.ClusterInfo.Name, state.ClusterInfo.Hash)
}

func (l LBcluster) Build() error {
	var currentClusterInfo *pb.ClusterInfo
	var currentDNS *pb.DNS
	var currentNodeIPs []string

	// Check if current cluster was defined, to avoid access of unrefferenced memory
	if l.CurrentState != nil {
		currentClusterInfo = l.CurrentState.ClusterInfo
		currentDNS = l.CurrentState.Dns
		currentNodeIPs = getNodeIPs(l.CurrentState.ClusterInfo.NodePools)
	}

	clusterBuilder := clusterBuilder.ClusterBuilder{
		DesiredClusterInfo: l.DesiredState.ClusterInfo,
		CurrentClusterInfo: currentClusterInfo,

		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB,
		Metadata: map[string]any{
			"roles": l.DesiredState.Roles,
		},
	}

	err := clusterBuilder.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the LB cluster %s : %w", l.DesiredState.ClusterInfo.Name, err)
	}

	nodeIPs := getNodeIPs(l.DesiredState.ClusterInfo.NodePools)
	dns := DNS{
		ClusterName:    l.DesiredState.ClusterInfo.Name,
		ClusterHash:    l.DesiredState.ClusterInfo.Hash,
		CurrentNodeIPs: currentNodeIPs,
		DesiredNodeIPs: nodeIPs,
		CurrentDNS:     currentDNS,
		DesiredDNS:     l.DesiredState.Dns,
		ProjectName:    l.ProjectName,
	}
	endpoint, err := dns.CreateDNSRecords()
	if err != nil {
		return fmt.Errorf("error while creating the DNS for %s : %w", l.DesiredState.ClusterInfo.Name, err)
	}

	l.DesiredState.Dns.Endpoint = endpoint

	return nil
}

func (l LBcluster) Destroy() error {
	group := errgroup.Group{}

	group.Go(func() error {
		cluster := clusterBuilder.ClusterBuilder{
			CurrentClusterInfo: l.CurrentState.ClusterInfo,
			ProjectName:        l.ProjectName,
			ClusterType:        pb.ClusterType_LB,
		}
		return cluster.DestroyNodepools()
	})

	group.Go(func() error {
		dns := DNS{
			ClusterName:    l.CurrentState.ClusterInfo.Name,
			ClusterHash:    l.CurrentState.ClusterInfo.Hash,
			CurrentNodeIPs: getNodeIPs(l.CurrentState.ClusterInfo.NodePools),
			CurrentDNS:     l.CurrentState.Dns,
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
