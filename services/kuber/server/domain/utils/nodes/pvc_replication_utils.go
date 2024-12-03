package nodes

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/kubectl"
	"gopkg.in/yaml.v3"
)

type ReplicaList struct {
	Items []LonghornReplica `yaml:"items"`
}

type LonghornReplica struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`

	Status struct {
		InstanceManagerName string `yaml:"instanceManagerName"`
		CurrentState        string `yaml:"currentState"`
		Started             bool   `yaml:"started"`
	} `yaml:"status"`

	Spec struct {
		NodeID string `yaml:"nodeID"`
	} `yaml:"spec"`
}

func deleteReplicaOnNode(kc kubectl.Kubectl, node string) error {
	out, err := kc.KubectlGet("replicas.longhorn.io", "-n", longhornNamespace, "-o", "yaml")
	if err != nil {
		return fmt.Errorf("failed to list all replicas : %w", err)
	}

	var replicaList ReplicaList
	if err := yaml.Unmarshal(out, &replicaList); err != nil {
		return fmt.Errorf("failed unmarshal kubectl output : %w", err)
	}

	i := slices.IndexFunc(replicaList.Items, func(replica LonghornReplica) bool {
		del := replica.Spec.NodeID == node
		del = del && replica.Status.CurrentState == "stopped"
		del = del && !replica.Status.Started
		del = del && replica.Status.InstanceManagerName == ""
		return del
	})
	if i < 0 {
		return nil
	}

	return kc.KubectlDeleteResource("replicas.longhorn.io", replicaList.Items[i].Metadata.Name, "-n", longhornNamespace)
}
