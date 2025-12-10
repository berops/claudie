package service

import (
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

// Based on the values in the [spec.K8Scluster.InstallationProxy] of the provided state
// returns the proxy settings via a [spec.Proxy] and true if the proxy should be used
// within that [spec.Clusters] state. Otherwise returns false.
func UsesProxy(k8s *spec.K8SclusterV2) bool {
	useProxy := k8s.InstallationProxy.Mode == ProxyDefaultMode && hasHetznerNode(k8s)
	useProxy = useProxy || k8s.InstallationProxy.Mode == ProxyOnMode
	return useProxy
}

func hasHetznerNode(k8s *spec.K8SclusterV2) bool {
	for _, np := range k8s.GetClusterInfo().GetNodePools() {
		np := np.GetDynamicNodePool()
		if np == nil {
			continue
		}

		if name := strings.ToLower(np.Provider.CloudProviderName); name == "hetzner" {
			return true
		}
	}
	return false
}
