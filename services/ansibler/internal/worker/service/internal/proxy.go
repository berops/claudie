package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

// Default values for the NoProxy list.
const (
	defaultHttpProxyUrl = "http://proxy.claudie.io:8880"

	// 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR.
	// 10.0.0.0/8, 172.16.0.0/12 and 192.168.0.0/16 are the RFC 1918 private ranges, which the
	// proxy cannot reach anyway. They also cover the private IPs of the cluster's own nodes.
	noProxyDefault = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
)

type Proxy struct {
	HttpProxyUrl string
	NoProxyList  string
}

// Returns the filled out [Proxy] structure based on the passed in
// values. The provided [*spec.K8Scluster] and [][*spec.LBCluster]
// are only traversed to collect the public IPs of the nodes and the
// DNS endpoints of the loadbalancers to form the NoProxyList. Private
// IPs are covered by the RFC 1918 ranges in the defaults. The **actuall Proxy arguments are
// read from the passed in [*spec.InstallationProxy] parameter**. This
// could be the same one as the one within the passed in [*spec.K8Scluster]
// or could be a completely different one.
func HttpProxyUrlAndNoProxyList(
	k8s *spec.K8Scluster,
	lbs []*spec.LBcluster,
	installationProxy *spec.InstallationProxy,
) Proxy {
	httpProxyUrl, noProxyList := defaultHttpProxyUrl, createNoProxyList(k8s, lbs, installationProxy)
	if installationProxy.Endpoint != "" {
		httpProxyUrl = installationProxy.Endpoint
	}
	return Proxy{
		HttpProxyUrl: httpProxyUrl,
		NoProxyList:  noProxyList,
	}
}

func createNoProxyList(
	k8s *spec.K8Scluster,
	lbs []*spec.LBcluster,
	installationProxy *spec.InstallationProxy,
) string {
	noProxyList := noProxyDefault
	if userNoProxy := installationProxy.NoProxy; userNoProxy != "" {
		noProxyList = fmt.Sprintf("%v,%v", noProxyList, userNoProxy)
	}

	for _, np := range k8s.ClusterInfo.NodePools {
		for _, node := range np.Nodes {
			noProxyList = fmt.Sprintf("%s,%s", noProxyList, node.Public)
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
				noProxyList = fmt.Sprintf("%s,%s", noProxyList, node.Public)
			}
		}
	}

	// if "svc" isn't in noProxyList the admission webhooks will fail, because they will be routed to proxy
	// "metadata,metadata.google.internal,169.254.169.254,metadata.google.internal." are required for GCP VMs
	return fmt.Sprintf("%s,svc,metadata,metadata.google.internal,169.254.169.254,metadata.google.internal.,", noProxyList)
}
