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

func DetermineLBApiEndpointChange(currentLbs, desiredLbs []*spec.LBcluster) (string, string, spec.ApiEndpointChangeState) {
	var first *spec.LBcluster
	desired := make(map[string]*spec.LBcluster)
	for _, lb := range desiredLbs {
		if lb.HasApiRole() {
			desired[lb.ClusterInfo.Id()] = lb
			if first == nil {
				first = lb
			}
		}
	}

	if current := FindAssignedLbApiEndpoint(currentLbs); current != nil {
		if len(desired) == 0 && current.Dns == nil { // current state failed to build dns, but lb was deleted.
			return "", "", spec.ApiEndpointChangeState_NoChange
		}
		if len(desired) == 0 {
			return current.ClusterInfo.Id(), "", spec.ApiEndpointChangeState_DetachingLoadBalancer
		}
		if current.Dns == nil { // current state failed to build dns but there is at least one cluster in desired state.
			return "", first.ClusterInfo.Id(), spec.ApiEndpointChangeState_AttachingLoadBalancer
		}
		if desired, ok := desired[current.ClusterInfo.Id()]; ok {
			if current.Dns.Endpoint != desired.Dns.Endpoint {
				return current.ClusterInfo.Id(), desired.ClusterInfo.Id(), spec.ApiEndpointChangeState_EndpointRenamed
			}
			return "", "", spec.ApiEndpointChangeState_NoChange
		}
		return current.ClusterInfo.Id(), first.ClusterInfo.Id(), spec.ApiEndpointChangeState_MoveEndpoint
	} else {
		if len(desired) == 0 {
			return "", "", spec.ApiEndpointChangeState_NoChange
		}
		return "", first.ClusterInfo.Id(), spec.ApiEndpointChangeState_AttachingLoadBalancer
	}
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
