package spec

import (
	"maps"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/sanitise"
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
	NodepoolKey         LabelKey = "claudie.io~1nodepool"
	ProviderKey         LabelKey = "claudie.io~1provider"
	ProviderInstanceKey LabelKey = "claudie.io~1provider-instance"
	NodeTypeKey         LabelKey = "claudie.io~1node-type"
	KubernetesZoneKey   LabelKey = "topology.kubernetes.io~1zone"
	KubernetesRegionKey LabelKey = "topology.kubernetes.io~1region"
	KubernetesOsKey     LabelKey = "kubernetes.io~1os"
	KubernetesArchKey   LabelKey = "kubernetes.io~1arch"
	KubeoneOsKey        LabelKey = "v1.kubeone.io~1operating-system"
	SpotKey             LabelKey = "claudie.io~1spot"
)

const (
	ControlPlane = "node-role.kubernetes.io/control-plane"
	// SpotTaintKey is the taint key marking GCP Spot nodes (effect NoSchedule).
	SpotTaintKey = "claudie.io/spot"
	// SpotValue is the value of the spot label and taint.
	SpotValue = "true"
)

// GetAllLabels returns default labels with their theoretical values for the specified nodepool,
// While also allowing to pass in additional labels to be set together with the [spec.NodePool.Labels].
func (np *NodePool) AllLabels(additionalLabels map[string]string) (map[string]string, error) {
	m := make(map[string]string, len(np.Labels)+9)

	// Apply default static nodepool labels.
	if n := np.GetStaticNodePool(); n != nil {
		m[string(ProviderKey)] = sanitise.String(StaticNodepoolInfo_STATIC_PROVIDER.String())
		m[string(ProviderInstanceKey)] = sanitise.String(StaticNodepoolInfo_STATIC_PROVIDER.String())
		m[string(KubernetesZoneKey)] = sanitise.String(StaticNodepoolInfo_STATIC_ZONE.String())
		m[string(KubernetesRegionKey)] = sanitise.String(StaticNodepoolInfo_STATIC_REGION.String())
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
	m[string(NodepoolKey)] = np.Name
	m[string(NodeTypeKey)] = getNodeType(np)
	// Other labels.
	m[string(KubernetesOsKey)] = "linux" // Only Linux is supported.
	m[string(KubeoneOsKey)] = "ubuntu"   // Only supported Os

	// Dynamic nodepool data.
	if n := np.GetDynamicNodePool(); n != nil {
		m[string(ProviderKey)] = n.Provider.CloudProviderName
		m[string(ProviderInstanceKey)] = n.Provider.SpecName
		m[string(KubernetesZoneKey)] = sanitise.String(n.Zone)
		m[string(KubernetesRegionKey)] = sanitise.String(n.Region)

		if n.Spot {
			m[string(SpotKey)] = SpotValue
		}
		return m, nil
	}
	return m, nil
}

// GetAllTaints returns default taints with their theoretical values for the specified nodepool.
func (np *NodePool) AllTaints(additionalTaints []*Taint) []k8sV1.Taint {
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

	if n := np.GetDynamicNodePool(); n != nil && n.Spot {
		uniq[k8sV1.Taint{
			Key:    SpotTaintKey,
			Value:  SpotValue,
			Effect: k8sV1.TaintEffectNoSchedule,
		}] = struct{}{}
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
func getNodeType(np *NodePool) string {
	if np.IsControl {
		return "control"
	}
	return "compute"
}

func escape(s string) string { return strings.ReplaceAll(s, "/", "~1") }
