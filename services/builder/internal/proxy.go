package builder

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

const (
	defaultHttpProxyUrl = "http://proxy.claudie.io:8880"
	noProxyDefault      = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12" // 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
)

func HttpProxyUrlAndNoProxyList(k8s *spec.K8Scluster, lbs []*spec.LBcluster, hasHetznerNodeFlag bool) (string, string) {
	httpProxyUrl, noProxyList := "", ""

	if k8s.InstallationProxy.Mode == "on" || (k8s.InstallationProxy.Mode == "default" && hasHetznerNodeFlag) {
		// The installation proxy is either turned on or in default mode with at least one Hetzner node in the k8s cluster.
		if k8s.InstallationProxy.Endpoint == "" {
			httpProxyUrl = defaultHttpProxyUrl
		} else {
			httpProxyUrl = k8s.InstallationProxy.Endpoint
		}
		noProxyList = createNoProxyList(k8s.InstallationProxy.NoProxy, k8s.ClusterInfo.NodePools, lbs)
	}

	return httpProxyUrl, noProxyList
}

func createNoProxyList(userNoProxy string, nodePools []*spec.NodePool, lbs []*spec.LBcluster) string {
	noProxyList := noProxyDefault
	if userNoProxy != "" {
		noProxyList = fmt.Sprintf("%v,%v", noProxyList, userNoProxy)
	}

	for _, np := range nodePools {
		for _, node := range np.Nodes {
			noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
		}
	}

	for _, lbCluster := range lbs {
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
	noProxyList = fmt.Sprintf("%s,svc,metadata,metadata.google.internal,169.254.169.254,metadata.google.internal.,", noProxyList)

	return noProxyList
}

func HasHetznerNode(k8s *spec.K8Scluster) bool {
	for _, np := range k8s.GetClusterInfo().GetNodePools() {
		if np.GetDynamicNodePool() != nil && np.GetDynamicNodePool().Provider.CloudProviderName == "hetzner" {
			return true
		}
	}
	return false
}

func DetermineProxyOperation(ctx *Context) spec.ProxyOp {
	defaultMode, offMode := "default", "off"
	hetznerInCurrent, hetznerInDesired := HasHetznerNode(ctx.CurrentCluster), HasHetznerNode(ctx.DesiredCluster)

	firstRun := ctx.CurrentCluster == nil || ctx.CurrentCluster.Kubeconfig == ""
	if firstRun {
		// The cluster wasn't build yet because currentState is nil or kubeconfig is not set
		// but we have to check if the proxy is turned on or in a default mode with Hetzner node in the desired state.
		desiredProxySettings := ctx.DesiredCluster.InstallationProxy

		if desiredProxySettings.Mode == defaultMode && hetznerInDesired {
			return spec.ProxyOp_MODIFIED
		} else if desiredProxySettings.Mode == "on" {
			return spec.ProxyOp_MODIFIED
		}
		return spec.ProxyOp_NONE
	} else {
		if ctx.CurrentCluster != nil && ctx.DesiredCluster != nil {
			currProxySettings := ctx.CurrentCluster.InstallationProxy
			desiredProxySettings := ctx.DesiredCluster.InstallationProxy

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
			case currProxySettings.Mode == offMode && desiredProxySettings.Mode == offMode:
				// The proxy is and was turned off in both cases.
				return spec.ProxyOp_NONE
			case currProxySettings.Mode == defaultMode && desiredProxySettings.Mode == defaultMode && !hetznerInCurrent && !hetznerInDesired:
				// The proxy is in default mode without Hetzner node in both cases.
				return spec.ProxyOp_NONE
			case currProxySettings.Mode == defaultMode && !hetznerInCurrent && desiredProxySettings.Mode == offMode:
				// The proxy was in default mode without Hetzner node and is turned off.
				return spec.ProxyOp_NONE
			case currProxySettings.Mode == offMode && desiredProxySettings.Mode == defaultMode && !hetznerInDesired:
				// The proxy was in off mode. Now it is in default mode without Hetzner node.
				return spec.ProxyOp_NONE
			}

			if desiredProxySettings.Mode == offMode {
				return spec.ProxyOp_OFF
			}
			return spec.ProxyOp_MODIFIED
		} else {
			return spec.ProxyOp_NONE
		}
	}
}
