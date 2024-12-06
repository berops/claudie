package nodes

import (
	"errors"
	"fmt"

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
		NodeID   string `yaml:"nodeID"`
		FailedAt string `yaml:"failedAt"`
	} `yaml:"spec"`
}

func removeReplicasOnDeletedNode(kc kubectl.Kubectl, node string) error {
	out, err := kc.KubectlGet("replicas.longhorn.io", "-n", longhornNamespace, "-o", "yaml")
	if err != nil {
		return fmt.Errorf("failed to list all replicas : %w", err)
	}

	var replicaList ReplicaList
	if err := yaml.Unmarshal(out, &replicaList); err != nil {
		return fmt.Errorf("failed unmarshal kubectl output : %w", err)
	}

	var errAll error
	for _, replica := range replicaList.Items {
		// https://github.com/longhorn/longhorn/blob/6cc47ec5e942f33b10f644a5eaf0970b650e27a7/deploy/longhorn.yaml#L3048
		// spec.NodeID is the node where the replica is on, this should
		// matched the deleted node.
		del := replica.Spec.NodeID == node
		del = del && replica.Status.CurrentState == "stopped"
		del = del && !replica.Status.Started
		del = del && replica.Status.InstanceManagerName == ""
		del = del && replica.Spec.FailedAt != ""

		if del {
			err := kc.KubectlDeleteResource("replicas.longhorn.io", replica.Metadata.Name, "-n", longhornNamespace)
			if err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to delete replica %s: %w", replica.Metadata.Name, err))
			}
		}
	}

	return errAll
}
