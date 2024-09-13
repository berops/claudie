package kube_eleven

import (
	"github.com/berops/claudie/proto/pb/spec"
)

type (
	// NodeInfo struct holds data necessary to define node in the node pool.
	NodeInfo struct {
		Node *spec.Node
		Name string
	}

	// NodepoolInfo struct holds data necessary to define nodes in kubeone
	// manifest.
	NodepoolInfo struct {
		Nodes             []*NodeInfo
		IsDynamic         bool
		NodepoolName      string
		Region            string
		Zone              string
		CloudProviderName string
		ProviderName      string
	}

	// templateData struct holds the data which will be used in creating
	// the Kubeone files from templates.
	templateData struct {
		APIEndpoint       string
		KubernetesVersion string
		ClusterName       string
		UtilizeHttpProxy  bool
		NoProxy           string
		HttpProxyUrl      string
		Nodepools         []*NodepoolInfo
	}
)
