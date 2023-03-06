package claudie_provider

import (
	"fmt"
	"time"

	"github.com/berops/claudie/services/kuber/server/nodes"
	k8sV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultNodeTemplate = &k8sV1.Node{
		Status: k8sV1.NodeStatus{
			Conditions: buildReadyConditions(),
		},
	}
)

func (c *ClaudieCloudProvider) getNodeGroupTemplateNodeInfo(nodeGroupId string) *k8sV1.Node {
	if ngc, ok := c.nodeGroupCache[nodeGroupId]; ok {
		node := defaultNodeTemplate
		node.Labels = c.nodeManager.GetLabels(ngc.nodepool)
		node.Status.Capacity = c.nodeManager.GetCapacity(ngc.nodepool)
		node.Status.Allocatable = node.Status.Capacity
		node.Spec.ProviderID = fmt.Sprintf(nodes.ProviderIdFormat, fmt.Sprintf("%s-N", ngc.nodepool.Name))
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