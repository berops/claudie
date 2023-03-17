package nodes

import (
	"fmt"

	"github.com/Berops/claudie/internal/kubectl"
	"gopkg.in/yaml.v3"
)

type K8sList[T any] struct {
	items []T `yaml:"items"`
}

type LonghornReplica struct {
	Metadata Metadata `yaml:"metadata"`
	Status   Status   `yaml:"status"`
}

type Metadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

type Status struct {
	OwnerID string `yaml:"ownerID"`
}

// getReplicas returns a map of nodes and slice of replicas they contain.
func getReplicas(kc kubectl.Kubectl) (map[string][]LonghornReplica, error) {
	var out []byte
	var err error
	if out, err = kc.KubectlGet("replicas.longhorn.io", "-n", longhornNamespace, "-o", "yaml"); err != nil {
		return nil, fmt.Errorf("failed to list all replicas : %w", err)
	}
	var replicaList K8sList[LonghornReplica]
	if err := yaml.Unmarshal(out, &replicaList); err != nil {
		return nil, fmt.Errorf("failed unmarshal kubectl output : %w", err)
	}

	m := make(map[string][]LonghornReplica, len(replicaList.items))
	for _, r := range replicaList.items {
		m[r.Status.OwnerID] = append(m[r.Status.OwnerID], r)
	}
	return m, nil
}

// deleteReplica deletes a replica from a node.
func deleteReplica(r LonghornReplica, kc kubectl.Kubectl) error {
	return kc.KubectlDeleteResource("replicas.longhorn.io", r.Metadata.Name, "-n", longhornNamespace)
}
