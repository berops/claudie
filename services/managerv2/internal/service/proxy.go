package service

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/proto/pb/spec"
)

// Known Proxy modes as defined by the InputManifest spec.
const (
	// Default Indicates that if any hetzner node the proxy will be used.
	ProxyDefaultMode = "default"

	// Explicitly turn off the proxy.
	ProxyOffMode = "off"

	// Explicitly turn on the proxy.
	ProxyOnMode = "on"
)

// Default values for the NoProxy list.
const (
	defaultHttpProxyUrl = "http://proxy.claudie.io:8880"

	// 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
	noProxyDefault = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12"
)

func ProxyDiff(current, desired *spec.ClustersV2) *spec.Proxy {
	proxy := spec.Proxy{
		Op: proxyOp(current, desired),
	}

	// TODO: finish this.
	if proxy.Op == spec.ProxyOp

	return &proxy
}

func httpProxyUrlAndNoProxyList(clusters *spec.ClustersV2) (string, string) {
	httpProxyUrl, noProxyList := defaultHttpProxyUrl, createNoProxyList(clusters)
	if clusters.K8S.InstallationProxy.Endpoint != "" {
		httpProxyUrl = clusters.K8S.InstallationProxy.Endpoint
	}
	return httpProxyUrl, noProxyList
}

func createNoProxyList(clusters *spec.ClustersV2) string {
	noProxyList := noProxyDefault
	if userNoProxy := clusters.K8S.InstallationProxy.NoProxy; userNoProxy != "" {
		noProxyList = fmt.Sprintf("%v,%v", noProxyList, userNoProxy)
	}

	for _, np := range clusters.K8S.ClusterInfo.NodePools {
		for _, node := range np.Nodes {
			noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
		}
	}

	for _, lbCluster := range clusters.LoadBalancers.Clusters {
		noProxyList = fmt.Sprintf("%s,%s", noProxyList, lbCluster.Dns.Endpoint)
		for _, ep := range lbCluster.Dns.AlternativeNames {
			if ep.Endpoint != "" {
				noProxyList = fmt.Sprintf("%s,%s", noProxyList, ep.Endpoint)
			}
		}
		for _, np := range lbCluster.ClusterInfo.NodePools {
			for _, node := range np.Nodes {
				noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
			}
		}
	}

	// if "svc" isn't in noProxyList the admission webhooks will fail, because they will be routed to proxy
	// "metadata,metadata.google.internal,169.254.169.254,metadata.google.internal." are required for GCP VMs
	return fmt.Sprintf("%s,svc,metadata,metadata.google.internal,169.254.169.254,metadata.google.internal.,", noProxyList)
}

func proxyOp(current, desired *spec.ClustersV2) spec.Proxy_Op {
	var (
		hetznerInCurrent = hasHetznerNode(current)
		hetznerInDesired = hasHetznerNode(desired)

		cMode = current.K8S.InstallationProxy.Mode
		dMode = desired.K8S.InstallationProxy.Mode
	)

	// The following use cases represents scenarios when the proxy envs have to be updated:
	// The proxy mode is on in both current and the desired state.
	// The proxy mode is default in both current and the desired state. Both states have at least one Hetzner node.
	// The proxy mode is default in both current and the desired state. The desired state has at least one Hetzner node. The current state doesn't have any Hetzner nodes.
	// The proxy mode is default in both current and the desired state. The desired state doesn't have any Hetzner nodes. The current state have at least one Hetzner node.
	// The proxy mode is default in both current and the desired state. Both states don't have any Hetzner nodes.
	// The proxy mode is default in the current state. The current state doesn't have any Hetzner nodes. The proxy is turned on in the desired state.
	// The proxy mode is default in the current state. The current state has at least one Hetzner node. The proxy is turned off in the desired state.
	// The proxy mode is default in the current state. The current state has at least one Hetzner node. The proxy is turned on in the desired state.
	// The proxy mode is off in the current state. The proxy modes is on in the desired state.
	// The proxy mode is on in the current state. The desired state has at least one Hetzner node and the proxy mode is default.
	// The proxy mode is on in the current state. The desired state doesn't have any Hetzner nodes and the proxy mode is default.
	// The proxy mode is off in the current state. The proxy mode is on in the desired state.
	// The proxy mode is off in the current state. The desired state has at least one Hetzner node and the proxy mode is default.

	// The following use cases represents scenarios when the proxy envs don't have to be updated:
	// The proxy mode is off in both current and the desired state.
	// The proxy mode is default in both current and the desired state. Both states don't have any Hetzner nodes.
	// The proxy mode is default in the current state. The current state doesn't have any Hetzner nodes. The proxy is turned off in the desired state.
	// The proxy mode is off in the current state. The desired state doesn't have any Hetzner nodes and the proxy mode is default.
	switch {
	case cMode == ProxyOffMode && dMode == ProxyOffMode:
		// The proxy is and was turned off in both cases.
		return spec.Proxy_NONE
	case cMode == ProxyDefaultMode && dMode == ProxyDefaultMode && !hetznerInCurrent && !hetznerInDesired:
		// The proxy is in default mode without Hetzner node in both cases.
		return spec.Proxy_NONE
	case cMode == ProxyDefaultMode && !hetznerInCurrent && dMode == ProxyOffMode:
		// The proxy was in default mode without Hetzner node and is turned off.
		return spec.Proxy_NONE
	case cMode == ProxyOffMode && dMode == ProxyDefaultMode && !hetznerInDesired:
		// The proxy was in off mode. Now it is in default mode without Hetzner node.
		return spec.Proxy_NONE
	}

	if dMode == ProxyOffMode {
		return spec.Proxy_OFF
	}

	return spec.Proxy_MODIFIED
}

func hasHetznerNode(clusters *spec.ClustersV2) bool {
	for _, np := range clusters.GetK8S().GetClusterInfo().GetNodePools() {
		np := np.GetDynamicNodePool()
		if np == nil {
			continue
		}

		if name := strings.ToLower(np.Provider.CloudProviderName); name == "hetzner" {
			return true
		}
	}

	for _, lb := range clusters.GetLoadBalancers().GetClusters() {
		for _, np := range lb.ClusterInfo.NodePools {
			np := np.GetDynamicNodePool()
			if np == nil {
				continue
			}

			if name := strings.ToLower(np.Provider.CloudProviderName); name == "hetzner" {
				return true
			}
		}
	}

	return false
}
