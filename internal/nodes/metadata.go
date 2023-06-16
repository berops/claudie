package nodes

import (
	"strings"

	"github.com/berops/claudie/proto/pb"

	k8sV1 "k8s.io/api/core/v1"
)

type LabelKey string

const (
	Nodepool         LabelKey = "claudie.io/nodepool"
	Provider         LabelKey = "claudie.io/provider"
	ProviderInstance LabelKey = "claudie.io/provider-instance"
	NodeType         LabelKey = "claudie.io/node-type"
	KubernetesZone   LabelKey = "topology.kubernetes.io/zone"
	KubernetesRegion LabelKey = "topology.kubernetes.io/region"
	KubernetesOs     LabelKey = "kubernetes.io/os"
	KubernetesArch   LabelKey = "kubernetes.io/arch"
	KubeoneOs        LabelKey = "v1.kubeone.io/operating-system"
)

const (
	ControlPlane = "node-role.kubernetes.io/control-plane"
	NoSchedule   = "NoSchedule"
)

// GetAllLabels returns default labels with their theoretical values for the specified nodepool.
func GetAllLabels(np *pb.NodePool) map[string]string {
	m := make(map[string]string)
	// Claudie assigned labels.
	m[string(Nodepool)] = np.Name
	m[string(Provider)] = np.GetDynamicNodePool().Provider.CloudProviderName
	m[string(ProviderInstance)] = np.GetDynamicNodePool().Provider.SpecName
	m[string(NodeType)] = getNodeType(np)
	m[string(KubernetesZone)] = sanitiseString(np.GetDynamicNodePool().Zone)
	m[string(KubernetesRegion)] = sanitiseString(np.GetDynamicNodePool().Region)
	// Other labels.
	m[string(KubernetesOs)] = "linux"   // Only Linux is supported.
	m[string(KubernetesArch)] = "amd64" // TODO add arch https://github.com/berops/claudie/issues/665
	m[string(KubeoneOs)] = "ubuntu"     // Only supported Os

	// Add custom user defined labels.
	for k, v := range np.Labels {
		m[k] = v
	}

	return m
}

// GetAllTaints returns default taints with their theoretical values for the specified nodepool.
func GetAllTaints(np *pb.NodePool) []k8sV1.Taint {
	taints := make([]k8sV1.Taint, 0)
	// Claudie assigned taints.
	if np.IsControl {
		taints = append(taints, k8sV1.Taint{Key: ControlPlane, Value: "", Effect: NoSchedule})
	}

	// Add custom user defined taints.
	for _, t := range np.Taints {
		taints = append(taints, k8sV1.Taint{Key: t.Key, Value: t.Value, Effect: k8sV1.TaintEffect(t.Effect)})
	}

	return taints
}

// GetCustomLabels returns default labels with their theoretical values for the specified nodepool.
func GetCustomLabels(np *pb.NodePool) map[string]string {
	m := make(map[string]string)
	// Claudie assigned labels.
	m[string(Nodepool)] = np.Name
	m[string(Provider)] = np.GetDynamicNodePool().Provider.CloudProviderName
	m[string(ProviderInstance)] = np.GetDynamicNodePool().Provider.SpecName
	m[string(NodeType)] = getNodeType(np)
	m[string(KubernetesZone)] = np.GetDynamicNodePool().Zone
	m[string(KubernetesRegion)] = np.GetDynamicNodePool().Region

	// Add custom user defined labels.
	for k, v := range np.Labels {
		m[k] = v
	}

	return m
}

// GetCustomTaints returns default taints with their theoretical values for the specified nodepool.
func GetCustomTaints(np *pb.NodePool) []k8sV1.Taint {
	taints := make([]k8sV1.Taint, 0)

	// Add custom user defined taints.
	for _, t := range np.Taints {
		taints = append(taints, k8sV1.Taint{Key: t.Key, Value: t.Value, Effect: k8sV1.TaintEffect(t.Effect)})
	}

	return taints
}

// getNodeType returns node type as a string value.
func getNodeType(np *pb.NodePool) string {
	if np.IsControl {
		return "control"
	}
	return "compute"
}

// sanitiseString replaces all white spaces and ":" in the string to "-".
func sanitiseString(s string) string {
	// convert to lower case
	sanitised := strings.ToLower(s)
	// replace all white space with "-"
	sanitised = strings.ReplaceAll(sanitised, " ", "-")
	// replace all ":" with "-"
	sanitised = strings.ReplaceAll(sanitised, ":", "-")
	// replace all "_" with "-"
	sanitised = strings.ReplaceAll(sanitised, "_", "-")
	return sanitised
}
