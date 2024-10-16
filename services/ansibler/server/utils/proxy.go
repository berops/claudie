package utils

import (
	"fmt"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
)

const (
	defaultHttpProxyMode = "default"
	defaultHttpProxyUrl  = "http://proxy.claudie.io:8880"
	noProxyDefault       = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12" // 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
)

func GetHttpProxyUrlAndNoProxyList(desiredK8sClusterInfo *spec.ClusterInfo, desiredLbs []*spec.LBcluster) (string, string) {
	var httpProxyUrl, noProxyList string
	hasHetznerNodeFlag := hasHetznerNode(desiredK8sClusterInfo)
	httpProxyMode := commonUtils.GetEnvDefault("HTTP_PROXY_MODE", defaultHttpProxyMode)

	if httpProxyMode == "off" || (httpProxyMode == "default" && !hasHetznerNodeFlag) {
		// set empty proxy env variables when proxy is off or a k8s cluster doesn't have any hetzner nodes in proxy default mode.
		httpProxyUrl = ""
		noProxyList = ""
	} else {
		noProxyList = createNoProxyList(desiredK8sClusterInfo.GetNodePools(), desiredLbs)
		httpProxyUrl = commonUtils.GetEnvDefault("HTTP_PROXY_URL", defaultHttpProxyUrl)
	}

	return httpProxyUrl, noProxyList
}

func createNoProxyList(desiredNodePools []*spec.NodePool, desiredLbs []*spec.LBcluster) string {
	noProxyList := noProxyDefault

	for _, np := range desiredNodePools {
		for _, node := range np.Nodes {
			noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
		}
	}

	for _, lbCluster := range desiredLbs {
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

func hasHetznerNode(desiredK8sClusterInfo *spec.ClusterInfo) bool {
	desiredNodePools := desiredK8sClusterInfo.GetNodePools()
	for _, np := range desiredNodePools {
		if np.GetDynamicNodePool() != nil && np.GetDynamicNodePool().Provider.CloudProviderName == "hetzner" {
			return true
		}
	}

	return false
}
