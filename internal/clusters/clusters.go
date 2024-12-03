package clusters

import "github.com/berops/claudie/proto/pb/spec"

func IndexLoadbalancerByName(target string, clusters []*spec.LBcluster) int {
	for i, cluster := range clusters {
		if cluster.ClusterInfo.Name == target {
			return i
		}
	}
	return -1
}

// ExtractTargetPorts extracts target ports defined inside the role in the LoadBalancer.
func ExtractTargetPorts(lbs []*spec.LBcluster) []int {
	ports := make(map[int32]struct{})

	var result []int
	for _, c := range lbs {
		for _, role := range c.Roles {
			if _, ok := ports[role.TargetPort]; !ok {
				result = append(result, int(role.TargetPort))
			}
			ports[role.TargetPort] = struct{}{}
		}
	}

	return result
}
