package claudie_provider

import (
	"time"

	k8sV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultNodeTemplate = &k8sV1.Node{
		Status: k8sV1.NodeStatus{
			Conditions: buildReadyConditions(),
		},
	}

	// All labels Claudie puts on nodes by default.
	claudieLabels = []string{
		"claudie.io/nodepool",
		"claudie.io/provider",
		"claudie.io/provider-instance",
		"claudie.io/node-type",
		"topology.kubernetes.io/zone",
		"topology.kubernetes.io/region",
	}

	// All default labels
	defaultLabels = []string{
		"kubernetes.io/os",
		"kubernetes.io/hostname",
		"kubernetes.io/arch",
		"v1.kubeone.io/operating-system",
	}
)

func (c *ClaudieCloudProvider) getNodeGroupTemplateNodeInfo(nodeGroupId string) *k8sV1.Node {
	if np, ok := c.nodepoolCache[nodeGroupId]; ok {
		node := defaultNodeTemplate
		node.Labels = c.nodeManager.GetLabels(np)
		node.Status.Capacity = c.nodeManager.GetCapacity(np)
		node.Status.Allocatable = node.Status.Capacity
		return node
	}
	return nil
}

func buildReadyConditions() []k8sV1.NodeCondition {
	lastTransition := time.Now().Add(-time.Minute)
	return []k8sV1.NodeCondition{
		{
			Type:               k8sV1.NodeReady,
			Status:             k8sV1.ConditionTrue,
			LastTransitionTime: metaV1.Time{Time: lastTransition},
		},
		{
			Type:               k8sV1.NodeNetworkUnavailable,
			Status:             k8sV1.ConditionFalse,
			LastTransitionTime: metaV1.Time{Time: lastTransition},
		},
		{
			Type:               k8sV1.NodeDiskPressure,
			Status:             k8sV1.ConditionFalse,
			LastTransitionTime: metaV1.Time{Time: lastTransition},
		},
		{
			Type:               k8sV1.NodeMemoryPressure,
			Status:             k8sV1.ConditionFalse,
			LastTransitionTime: metaV1.Time{Time: lastTransition},
		},
	}
}
