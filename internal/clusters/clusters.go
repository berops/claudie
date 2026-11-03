package clusters

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

func IndexLoadbalancerById(target string, clusters []*spec.LBcluster) int {
	for i, cluster := range clusters {
		if cluster.ClusterInfo.Id() == target {
			return i
		}
	}
	return -1
}

func FindLoadBalancerApiServerEndpoint(clusters []*spec.LBcluster) *spec.LBcluster {
	for _, lb := range clusters {
		if lb.IsApiEndpoint() {
			return lb
		}
	}
	return nil
}

// FindControlNodeApiServerEndpoint searches for a nodepool that has the control node representing the Api endpoint of the cluster.
func FindControlNodeApiServerEndpoint(nodepools []*spec.NodePool) (*spec.NodePool, *spec.Node) {
	for _, np := range nodepools {
		if np.IsControl {
			if node := np.EndpointNode(); node != nil {
				return np, node
			}
		}
	}
	return nil, nil
}

// ControlPlaneEndpointHost returns the host, without the port, of the control plane endpoint:
// the single stable address in front of the api servers of the cluster, as kubeadm names it
// (controlPlaneEndpoint). It is the DNS endpoint of the loadbalancer with the ApiServer role,
// or without such a loadbalancer, the public IP of the control node marked as the api server
// endpoint by the manager.
//
// Every place that needs this address, e.g. the kubeadm controlPlaneEndpoint or the API
// server address used by Cilium, has to use this function so that they always agree.
func ControlPlaneEndpointHost(k8s *spec.K8Scluster, lbs []*spec.LBcluster) (string, error) {
	if lb := FindLoadBalancerApiServerEndpoint(lbs); lb != nil {
		if ep := lb.GetDns().GetEndpoint(); ep != "" {
			return ep, nil
		}
		return "", fmt.Errorf("loadbalancer %q is assigned as the api endpoint but has no dns endpoint", lb.ClusterInfo.Id())
	}

	_, node := FindControlNodeApiServerEndpoint(k8s.GetClusterInfo().GetNodePools())
	if node == nil {
		return "", errors.New("malformed k8s state, no loadbalancer attached with api role nor any control plane node has api server role")
	}

	if node.Public == "" {
		return "", fmt.Errorf("api endpoint node %q has no public ip", node.Name)
	}

	return node.Public, nil
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
