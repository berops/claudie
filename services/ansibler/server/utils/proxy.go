package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

const (
	defaultHttpProxyUrl = "http://proxy.claudie.io:8880"
	noProxyDefault      = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12" // 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
)

type (
	ProxyInventoryFileParameters struct {
		K8sNodepools NodePools
		ClusterID    string
		NoProxyList  string
		HttpProxyUrl string
	}
)

func GetHttpProxyUrlAndNoProxyList(k8sClusterInfo *spec.ClusterInfo, lbs []*spec.LBcluster, k8sInstallationProxy *spec.InstallationProxy) (string, string) {
	var httpProxyUrl, noProxyList = "", ""
	hasHetznerNodeFlag := hasHetznerNode(k8sClusterInfo)

	if (k8sInstallationProxy != nil && k8sInstallationProxy.Enabled) || hasHetznerNodeFlag {
		httpProxyUrl = k8sInstallationProxy.Host
		noProxyList = createNoProxyList(k8sClusterInfo.GetNodePools(), lbs)
	}

	if k8sInstallationProxy != nil {
		if k8sInstallationProxy.Enabled {
			// Set proxy env variables when proxy is on.
			if k8sInstallationProxy.Host == "" {
				httpProxyUrl = defaultHttpProxyUrl
			} else {
				httpProxyUrl = k8sInstallationProxy.Host
			}
			noProxyList = createNoProxyList(k8sClusterInfo.GetNodePools(), lbs)
		} else {
			// Set empty proxy env variables when proxy is off.
			httpProxyUrl = ""
			noProxyList = ""
		}
	} else {
		hasHetznerNodeFlag := hasHetznerNode(k8sClusterInfo)

		if hasHetznerNodeFlag {
			// Set proxy env variables when k8s cluster has at least one hetzner node.
			httpProxyUrl = defaultHttpProxyUrl
			noProxyList = createNoProxyList(k8sClusterInfo.GetNodePools(), lbs)
		}
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

func hasHetznerNode(k8sClusterInfo *spec.ClusterInfo) bool {
	nodePools := k8sClusterInfo.GetNodePools()
	for _, np := range nodePools {
		if np.GetDynamicNodePool() != nil && np.GetDynamicNodePool().Provider.CloudProviderName == "hetzner" {
			return true
		}
	}

	return false
}
