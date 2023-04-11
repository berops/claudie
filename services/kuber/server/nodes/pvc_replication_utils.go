package nodes

import (
	"fmt"

	"github.com/berops/claudie/internal/kubectl"
	"gopkg.in/yaml.v3"
)

type K8sList[T any] struct {
	Items []T `yaml:"items"`
}

type LonghornReplica struct {
	Metadata Metadata    `yaml:"metadata"`
	Status   Status      `yaml:"status"`
	Spec     ReplicaSpec `yaml:"spec"`
}

type LonghornVolume struct {
	Metadata Metadata   `yaml:"metadata"`
	Spec     VolumeSpec `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type ReplicaSpec struct {
	VolumeName string `yaml:"volumeName"`
}

type VolumeSpec struct {
	NumberOfReplicas int `yaml:"numberOfReplicas"`
}

type Status struct {
	OwnerID string `yaml:"ownerID"`
}

const (
	replicas              = "replicas.longhorn.io"
	volumes               = "volumes.longhorn.io"
	patchNumberOfReplicas = "{\"spec\":{\"numberOfReplicas\":%d}}"
)

// getVolumes returns a map of volumes currently in the cluster.
func getVolumes(kc kubectl.Kubectl) (map[string]LonghornVolume, error) {
	out, err := kc.KubectlGet(volumes, "-n", longhornNamespace, "-o", "yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to list all volumes : %w", err)
	}
	var volumeList K8sList[LonghornVolume]
	if err := yaml.Unmarshal(out, &volumeList); err != nil {
		return nil, fmt.Errorf("failed unmarshal kubectl output : %w", err)
	}

	m := make(map[string]LonghornVolume, len(volumeList.Items))
	for _, v := range volumeList.Items {
		m[v.Metadata.Name] = v
	}
	return m, nil
}

// getReplicas returns a map of nodes and slice of replicas they contain.
func getReplicas(kc kubectl.Kubectl) (map[string][]LonghornReplica, error) {
	out, err := kc.KubectlGet(replicas, "-n", longhornNamespace, "-o", "yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to list all replicas : %w", err)
	}
	var replicaList K8sList[LonghornReplica]
	if err := yaml.Unmarshal(out, &replicaList); err != nil {
		return nil, fmt.Errorf("failed unmarshal kubectl output : %w", err)
	}

	m := make(map[string][]LonghornReplica, len(replicaList.Items))
	for _, r := range replicaList.Items {
		m[r.Status.OwnerID] = append(m[r.Status.OwnerID], r)
	}
	return m, nil
}

// deleteReplica deletes a replica from a node.
func deleteReplica(r LonghornReplica, kc kubectl.Kubectl) error {
	return kc.KubectlDeleteResource(replicas, r.Metadata.Name, "-n", longhornNamespace)
}

// increaseReplicaCount increases number of replicas for longhorn volume by 1, via kubectl patch.
func increaseReplicaCount(v LonghornVolume, kc kubectl.Kubectl) error {
	return kc.KubectlPatch(volumes, v.Metadata.Name, fmt.Sprintf(patchNumberOfReplicas, v.Spec.NumberOfReplicas+1), "-n", longhornNamespace, "--type", "merge")
}

// revertReplicaCount sets the number of replicas for longhorn volume to the original value, taken from the v, via kubectl patch
func revertReplicaCount(v LonghornVolume, kc kubectl.Kubectl) error {
	return kc.KubectlPatch(volumes, v.Metadata.Name, fmt.Sprintf(patchNumberOfReplicas, v.Spec.NumberOfReplicas), "-n", longhornNamespace, "--type", "merge")
}
