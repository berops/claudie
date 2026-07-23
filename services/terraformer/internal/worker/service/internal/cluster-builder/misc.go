package cluster_builder

import "fmt"

// ClusterType enumerates supported clusters that can be build
// using the [ClusterBuilder]
type ClusterType string

const (
	// String identifier of a kubernetes cluster type.
	KubernetesCluster ClusterType = "K8s"

	// String identifier of a loadbalancer cluster type.
	LoadbalancerCluster ClusterType = "LB"
)

const (
	// Root directory of the external templates.
	TemplatesRootDir = "services/terraformer/templates"

	// Output directory where the external templates will
	// be generated to for each cluster.
	Output = "services/terraformer/clusters"

	// Cache directory for caching providers for Tofu.
	CacheDir = "services/terraformer/cache"
)

const (
	// Directory to which the common networking infrastructure is to be generated to.
	NetworkingGenTarget = "common-networking"

	// Directory to which individual nodepools are to be generated to.
	NodepoolsGenTarget = "nodepools"
)

func StateFileDnsSubKey(clusterId string) string { return fmt.Sprintf("%s-dns", clusterId) }

func StateFileNodePoolSubKey(clusterId, nodepoolName string) string {
	return fmt.Sprintf("%s-%s-%s", clusterId, NodepoolsGenTarget, nodepoolName)
}

func StateFileCommonInfrastructureSubKey(clusterId string) string {
	return fmt.Sprintf("%s-%s", clusterId, NetworkingGenTarget)
}

func tofuFormatLevel() func(any) string  { return func(a any) string { return "" } }
func tofuFormatCaller() func(any) string { return func(a any) string { return "" } }
