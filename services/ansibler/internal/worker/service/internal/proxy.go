package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

// Default values for the NoProxy list.
const (
	defaultHttpProxyUrl = "http://proxy.claudie.io:8880"

	// 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
	noProxyDefault = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12"
)

type Proxy struct {
	HttpProxyUrl string
	NoProxyList  string
}

func HttpProxyUrlAndNoProxyList(k8s *spec.K8Scluster, lbs []*spec.LBcluster) Proxy {
	httpProxyUrl, noProxyList := defaultHttpProxyUrl, createNoProxyList(k8s, lbs)
	if k8s.InstallationProxy.Endpoint != "" {
		httpProxyUrl = k8s.InstallationProxy.Endpoint
	}
	return Proxy{
		HttpProxyUrl: httpProxyUrl,
		NoProxyList:  noProxyList,
	}
}

func createNoProxyList(k8s *spec.K8Scluster, lbs []*spec.LBcluster) string {
	noProxyList := noProxyDefault
	if userNoProxy := k8s.InstallationProxy.NoProxy; userNoProxy != "" {
		noProxyList = fmt.Sprintf("%v,%v", noProxyList, userNoProxy)
	}

	for _, np := range k8s.ClusterInfo.NodePools {
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
	return fmt.Sprintf("%s,svc,metadata,metadata.google.internal,169.254.169.254,metadata.google.internal.,", noProxyList)
}
