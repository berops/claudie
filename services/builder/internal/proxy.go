package builder

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

const (
	defaultHttpProxyUrl = "http://proxy.claudie.io:8880"
	noProxyDefault      = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12" // 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
)

func GetHttpProxyUrlAndNoProxyList(k8sClusterInfo *spec.ClusterInfo, lbs []*spec.LBcluster, hasHetznerNodeFlag bool, k8sInstallationProxy *spec.InstallationProxy) (string, string) {
	var httpProxyUrl, noProxyList = "", ""

	if k8sInstallationProxy.Mode == *spec.InstallationProxy_On.Enum() || (k8sInstallationProxy.Mode == *spec.InstallationProxy_Default.Enum() && hasHetznerNodeFlag) {
		// The installation proxy is either turned on or in default mode with at least one Hetzner node in the k8s cluster.
		if k8sInstallationProxy.Endpoint == "" {
			httpProxyUrl = defaultHttpProxyUrl
		} else {
			httpProxyUrl = k8sInstallationProxy.Endpoint
		}
		noProxyList = createNoProxyList(k8sClusterInfo.GetNodePools(), lbs)
	}

	return httpProxyUrl, noProxyList
}

func createNoProxyList(nodePools []*spec.NodePool, lbs []*spec.LBcluster) string {
	noProxyList := noProxyDefault

	for _, np := range nodePools {
		for _, node := range np.Nodes {
			noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
		}
	}

	for _, lbCluster := range lbs {
		noProxyList = fmt.Sprintf("%s,%s", noProxyList, lbCluster.Dns.Endpoint)
		for _, np := range lbCluster.ClusterInfo.NodePools {
			for _, node := range np.Nodes {
				noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
			}
		}
	}

	// if "svc" isn't in noProxyList the admission webhooks will fail, because they will be routed to proxy
	// "metadata,metadata.google.internal,169.254.169.254,metadata.google.internal." are required for GCP VMs
	noProxyList = fmt.Sprintf("%s,svc,metadata,metadata.google.internal,169.254.169.254,metadata.google.internal.,", noProxyList)

	return noProxyList
}

func HasHetznerNode(k8sClusterInfo *spec.ClusterInfo) bool {
	nodePools := k8sClusterInfo.GetNodePools()
	for _, np := range nodePools {
		if np.GetDynamicNodePool() != nil && np.GetDynamicNodePool().Provider.CloudProviderName == "hetzner" {
			return true
		}
	}

	return false
}
