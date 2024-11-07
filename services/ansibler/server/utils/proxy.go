package utils

type (
	ProxyInventoryFileParameters struct {
		K8sNodepools NodePools
		ClusterID    string
		NoProxyList  string
		HttpProxyUrl string
	}
)
