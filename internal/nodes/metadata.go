package nodes

import (
	"maps"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/sanitise"
	"github.com/berops/claudie/proto/pb/spec"
	k8sV1 "k8s.io/api/core/v1"
)

type LabelKey string

const (
	ProviderId       = "claudie://"
	ProviderIdFormat = ProviderId + "%s"
)

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

// GetAllLabels returns default labels with their theoretical values for the specified nodepool,
// While also allowing to pass in additional labels to be set together with the [spec.NodePool.Labels].
func GetAllLabels(
	np *spec.NodePool,
	resolver ArchResolver,
	additionalLabels map[string]string,
) (map[string]string, error) {
	m := make(map[string]string, len(np.Labels)+9)

	// Apply default static nodepool labels.
	if n := np.GetStaticNodePool(); n != nil {
		m[string(Provider)] = sanitise.String(spec.StaticNodepoolInfo_STATIC_PROVIDER.String())
		m[string(ProviderInstance)] = sanitise.String(spec.StaticNodepoolInfo_STATIC_PROVIDER.String())
		m[string(KubernetesZone)] = sanitise.String(spec.StaticNodepoolInfo_STATIC_ZONE.String())
		m[string(KubernetesRegion)] = sanitise.String(spec.StaticNodepoolInfo_STATIC_REGION.String())
	}

	// In case the user wants to overwrite the static nodepool labels allow.
	// In case of dynamic nodepools, apply them first, so that if the user tries
	// to overwrite some of the claudie default related it will not allow it.
	for k, v := range np.Labels {
		m[escape(sanitise.String(k))] = sanitise.String(v)
	}
	for k, v := range additionalLabels {
		m[escape(sanitise.String(k))] = sanitise.String(v)
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
		m[string(KubernetesZone)] = sanitise.String(n.Zone)
		m[string(KubernetesRegion)] = sanitise.String(n.Region)

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
	return m, nil
}

// GetAllTaints returns default taints with their theoretical values for the specified nodepool.
func GetAllTaints(np *spec.NodePool, additionalTaints []*spec.Taint) []k8sV1.Taint {
	uniq := make(map[k8sV1.Taint]struct{}, len(np.Taints)+1)

	// Add custom user defined taints.
	for _, t := range np.Taints {
		t := k8sV1.Taint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: k8sV1.TaintEffect(t.Effect),
		}
		uniq[t] = struct{}{}
	}

	for _, t := range additionalTaints {
		t := k8sV1.Taint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: k8sV1.TaintEffect(t.Effect),
		}
		uniq[t] = struct{}{}
	}

	// Claudie assigned taints.
	if np.IsControl {
		t := k8sV1.Taint{
			Key:    ControlPlane,
			Value:  "",
			Effect: k8sV1.TaintEffectNoSchedule,
		}

		uniq[t] = struct{}{}
	}

	return slices.Collect(maps.Keys(uniq))
}

// getNodeType returns node type as a string value.
func getNodeType(np *spec.NodePool) string {
	if np.IsControl {
		return "control"
	}
	return "compute"
}

func escape(s string) string { return strings.ReplaceAll(s, "/", "~1") }
