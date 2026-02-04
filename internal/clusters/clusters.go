package clusters

import "github.com/berops/claudie/proto/pb/spec"

func IndexLoadbalancerById(target string, clusters []*spec.LBcluster) int {
	for i, cluster := range clusters {
		if cluster.ClusterInfo.Id() == target {
			return i
		}
	}
	return -1
}

func FindAssignedLbApiEndpoint(clusters []*spec.LBcluster) *spec.LBcluster {
	for _, lb := range clusters {
		if lb.IsApiEndpoint() {
			return lb
		}
	}
	return nil
}

func NodePublic(name string, cluster *spec.K8Scluster) string {
	for _, np := range cluster.GetClusterInfo().GetNodePools() {
		for _, n := range np.Nodes {
			if n.Name == name {
				return n.Public
			}
		}
	}
	return ""
}
