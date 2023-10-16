package nodes

import (
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	k8sV1 "k8s.io/api/core/v1"
)

type LabelKey string

// To properly escape / in JSONPatch, 
// it has to be replaced with ~1
// https://jsonpatch.com/#json-pointer
const (
	Nodepool         LabelKey = "claudie.io~1nodepool"
	Provider         LabelKey = "claudie.io~1provider"
	ProviderInstance LabelKey = "claudie.io~1provider-instance"
	NodeType         LabelKey = "claudie.io~1node-type"
	KubernetesZone   LabelKey = "topology.kubernetes.io~1zone"
	KubernetesRegion LabelKey = "topology.kubernetes.io~1region"
	KubernetesOs     LabelKey = "kubernetes.io~1os"
	KubernetesArch   LabelKey = "kubernetes.io~1arch"
	KubeoneOs        LabelKey = "v1.kubeone.io~1operating-system"
)

const (
	ControlPlane = "node-role.kubernetes.io/control-plane"
)

// GetAllLabels returns default labels with their theoretical values for the specified nodepool.
func GetAllLabels(np *pb.NodePool, resolver ArchResolver) (map[string]string, error) {
	m := make(map[string]string, len(np.Labels)+9)
	// Add custom user defined labels first in case user will try to overwrite Claudie default labels.
	for k, v := range np.Labels {
		m[k] = v
	}

	// Claudie assigned labels.
	m[string(Nodepool)] = np.Name
	m[string(NodeType)] = getNodeType(np)
	// Other labels.
	m[string(KubernetesOs)] = "linux" // Only Linux is supported.
	m[string(KubeoneOs)] = "ubuntu"   // Only supported Os
	// Dynamic nodepool data.
	if n := np.GetDynamicNodePool(); n != nil {
		m[string(Provider)] = n.Provider.CloudProviderName
		m[string(ProviderInstance)] = n.Provider.SpecName
		m[string(KubernetesZone)] = utils.SanitiseString(n.Zone)
		m[string(KubernetesRegion)] = utils.SanitiseString(n.Region)

		if resolver != nil {
			arch, err := resolver.Arch(np)
			if err != nil {
				return nil, err
			}

			// we only need to set this in case of the autoscaler.
			// The kubernetes.io/arch is otherwise set by kubelet.
			// https://github.com/berops/claudie/issues/665
			// https://github.com/berops/claudie/pull/934#issuecomment-1618318914
			m[string(KubernetesArch)] = string(arch)
		}

		return m, nil
	}
	// Static nodepool data.
	m[string(Provider)] = utils.SanitiseString(pb.StaticNodepoolInfo_STATIC_PROVIDER.String())
	m[string(ProviderInstance)] = utils.SanitiseString(pb.StaticNodepoolInfo_STATIC_PROVIDER.String())
	m[string(KubernetesZone)] = utils.SanitiseString(pb.StaticNodepoolInfo_STATIC_ZONE.String())
	m[string(KubernetesRegion)] = utils.SanitiseString(pb.StaticNodepoolInfo_STATIC_REGION.String())
	return m, nil
}

// GetAllTaints returns default taints with their theoretical values for the specified nodepool.
func GetAllTaints(np *pb.NodePool) []k8sV1.Taint {
	taints := make([]k8sV1.Taint, 0, len(np.Taints)+1)
	// Add custom user defined taints.
	for _, t := range np.Taints {
		if t.Key == ControlPlane && t.Effect == string(k8sV1.TaintEffectNoSchedule) && t.Value == "" {
			// Skipping as this is Claudie default taint
			continue
		}
		taints = append(taints, k8sV1.Taint{Key: t.Key, Value: t.Value, Effect: k8sV1.TaintEffect(t.Effect)})
	}

	// Claudie assigned taints.
	if np.IsControl {
		taints = append(taints, k8sV1.Taint{Key: ControlPlane, Value: "", Effect: k8sV1.TaintEffectNoSchedule})
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
