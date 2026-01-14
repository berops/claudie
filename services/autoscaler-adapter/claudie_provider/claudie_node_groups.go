package claudie_provider

// import (
// 	"context"
// 	"fmt"
// 	"slices"
// 	"strings"
// 	"time"

// 	k8sV1 "k8s.io/api/core/v1"
// 	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"

// 	"github.com/berops/claudie/internal/nodes"
// 	"github.com/berops/claudie/proto/pb/spec"
// 	"github.com/rs/zerolog/log"

// 	"google.golang.org/protobuf/proto"
// )

// // TODO: autoscaler: how to
// //
// //
// //
// // 1. We need to add a tagetSize to the AutoscalerConfig which would be the target Size of
// //    the external cluster-autoscaler.
// //
// //    This would didcate the count field in the generated nodes in the desired state since
// //    the count field will not be directly modified like it is with non-autoscaled nodepools
// //
// //    Further, we need to add a Status for each node that is one of Creating, Running, Deleting
// //    Where Creating is whenever a node is being created, Running is when its joined to the cluster
// //    Deleting is when it should be scheduled for deletion.
// //
// //
// //    This can then be make used of for increasing the targetSize we increase the targetsize of the
// //    autosclaer config, similarly we can decrease it if works has not been started yet, and similarly
// //    we can directly then mark nodes for deletion, specific nodes that autoscaler gives us.

// // TODO: we need to reverse the logic here, The manager should query the desired state of the
// // autoscaled nodepool each time it runs its reconciliation
// //
// // The target size seems to be the main source of truth and if I report
// // Check If I will need to state the nodes in Creation Deletion, Running states if that makes any difference.
// //

// // NodeGroupTargetSize returns the current target size of the node group. It is possible
// // that the number of nodes in Kubernetes is different at the moment but should be equal
// // to the size of a node group once everything stabilizes (new nodes finish startup and
// // registration or removed nodes are deleted completely).
// func (c *ClaudieCloudProvider) NodeGroupTargetSize(_ context.Context, req *protos.NodeGroupTargetSizeRequest) (*protos.NodeGroupTargetSizeResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got NodeGroupTargetSize request")

// 	if ngc, ok := c.managedPools[req.GetId()]; ok {
// 		log.Debug().Str("nodepool", req.GetId()).Msgf("Returning target size %d for nodepool", ngc.targetSize)
// 		return &protos.NodeGroupTargetSizeResponse{TargetSize: ngc.targetSize}, nil
// 	}

// 	return nil, fmt.Errorf("nodeGroup %s was not found", req.Id)
// }

// // NodeGroupIncreaseSize increases the size of the node group. To delete a node you need
// // to explicitly name it and use NodeGroupDeleteNodes. This function should wait until
// // node group size is updated.
// func (c *ClaudieCloudProvider) NodeGroupIncreaseSize(ctx context.Context, req *protos.NodeGroupIncreaseSizeRequest) (*protos.NodeGroupIncreaseSizeResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Str("nodepool", req.GetId()).Msgf("Got NodeGroupIncreaseSize request for nodepool by %d", req.GetDelta())

// 	// Find the nodepool.
// 	if ngc, ok := c.managedPools[req.GetId()]; ok {
// 		// Check & update the new Count.
// 		newCount := ngc.nodepool.GetDynamicNodePool().Count + req.GetDelta()
// 		if newCount > ngc.nodepool.GetDynamicNodePool().AutoscalerConfig.Max {
// 			return nil, fmt.Errorf("could not add new nodes, as that would be larger than max size of the nodepool; current size %d, requested delta %d", ngc.nodepool.GetDynamicNodePool().Count, req.GetDelta())
// 		}

// 		temp := proto.Clone(ngc.nodepool).(*spec.NodePool)
// 		temp.GetDynamicNodePool().Count = newCount

// 		if err := c.updateNodepool(ctx, temp); err != nil {
// 			return nil, fmt.Errorf("failed to update nodepool %s : %w", temp.Name, err)
// 		}
// 		if err := c.sendAutoscalerEvent(); err != nil {
// 			return nil, fmt.Errorf("failed to send autoscaler event %s : %w", temp.Name, err)
// 		}

// 		ngc.targetSize = newCount
// 		ngc.nodepool = temp

// 		return &protos.NodeGroupIncreaseSizeResponse{}, nil
// 	}

// 	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
// }

// // NodeGroupDeleteNodes deletes nodes from this node group (and also decreasing the size
// // of the node group with that). Error is returned either on failure or if the given node
// // doesn't belong to this node group. This function should wait until node group size is updated.
// func (c *ClaudieCloudProvider) NodeGroupDeleteNodes(ctx context.Context, req *protos.NodeGroupDeleteNodesRequest) (*protos.NodeGroupDeleteNodesResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()
// 	log.Info().Str("nodepool", req.GetId()).Msgf("Got NodeGroupDeleteNodes request for nodepool")
// 	// Find the nodepool.
// 	if ngc, ok := c.nodesCache[req.GetId()]; ok {
// 		// Check & update the new Count.
// 		newCount := ngc.nodepool.GetDynamicNodePool().GetCount() - int32(len(req.GetNodes()))
// 		if newCount < ngc.nodepool.GetDynamicNodePool().AutoscalerConfig.GetMin() {
// 			return nil, fmt.Errorf("could not remove nodes, as that would be smaller than min size of the nodepool; current size %d, requested removal %d", ngc.nodepool.GetDynamicNodePool().Count, len(req.GetNodes()))
// 		}

// 		temp := proto.Clone(ngc.nodepool).(*spec.NodePool)
// 		temp.GetDynamicNodePool().Count = newCount

// 		temp.Nodes = slices.DeleteFunc(temp.Nodes, func(node *spec.Node) bool {
// 			nodeId := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-%s-", c.clusterName, c.clusterHash))
// 			return slices.ContainsFunc(req.GetNodes(), func(n *protos.ExternalGrpcNode) bool { return n.Name == nodeId })
// 		})

// 		if err := c.updateNodepool(ctx, temp); err != nil {
// 			return nil, fmt.Errorf("failed to update nodepool %s : %w", temp.Name, err)
// 		}
// 		if err := c.sendAutoscalerEvent(); err != nil {
// 			return nil, fmt.Errorf("failed to send autoscaler event %s : %w", temp.Name, err)
// 		}

// 		ngc.targetSize = newCount
// 		ngc.nodepool = temp

// 		return &protos.NodeGroupDeleteNodesResponse{}, nil
// 	}
// 	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
// }

// // NodeGroupDecreaseTargetSize decreases the target size of the node group. This function
// // doesn't permit to delete any existing node and can be used only to reduce the request
// // for new nodes that have not been yet fulfilled. Delta should be negative. It is assumed
// // that cloud provider will not delete the existing nodes if the size when there is an option
// // to just decrease the target.
// func (c *ClaudieCloudProvider) NodeGroupDecreaseTargetSize(_ context.Context, req *protos.NodeGroupDecreaseTargetSizeRequest) (*protos.NodeGroupDecreaseTargetSizeResponse, error) {
// 	// TODO: or will we ?
// 	// Requests for new nodes are always fulfilled so we cannot decrease
// 	// the size of the nodepool without actually going through the deletion
// 	// of the nodes within a nodepool.
// 	//
// 	// Note(despire): Seems that this function is not mandatory to implement
// 	// looking at the implementations in:
// 	// https://github.com/search?q=repo%3Akubernetes%2Fautoscaler+DecreaseTargetSize&type=code
// 	//
// 	// Some providers avoid implementing it when they do not support decreasing the
// 	// targetsize without actuall deletion.
// 	return new(protos.NodeGroupDecreaseTargetSizeResponse), nil
// }

// // NodeGroupNodes returns a list of all nodes that belong to this node group.
// func (c *ClaudieCloudProvider) NodeGroupNodes(_ context.Context, req *protos.NodeGroupNodesRequest) (*protos.NodeGroupNodesResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got NodeGroupNodes request")

// 	instances := make([]*protos.Instance, 0)

// 	// TODO: it seems that Azure is returning some kind of placeholder nodes in here
// 	// that does not exist in the autoscaled group.
// 	if ngc, ok := c.nodesCache[req.GetId()]; ok {
// 		for _, node := range ngc.nodepool.Nodes {
// 			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-%s-", c.clusterName, c.clusterHash))
// 			instance := &protos.Instance{
// 				Id: fmt.Sprintf(nodes.ProviderIdFormat, nodeName),
// 				// TODO: seems like this is not mandatory and also stated as optional.
// 				// so we don't actually need this.
// 				// for example aws and azure don't set this at all.
// 				Status: &protos.InstanceStatus{
// 					// If there is an instance in the config.K8Scluster, then it is always considered running.
// 					InstanceState: protos.InstanceStatus_instanceRunning,
// 				},
// 			}
// 			instances = append(instances, instance)
// 		}
// 	}

// 	log.Debug().Msgf("NodeGroupForNodes returns %v", instances)

// 	return &protos.NodeGroupNodesResponse{Instances: instances}, nil
// }

// // NodeGroupTemplateNodeInfo returns a structure of an empty (as if just started) node,
// // with all of the labels, capacity and allocatable information. This will be used in
// // scale-up simulations to predict what would a new node look like if a node group was expanded.
// // Implementation optional.
// func (c *ClaudieCloudProvider) NodeGroupTemplateNodeInfo(_ context.Context, req *protos.NodeGroupTemplateNodeInfoRequest) (*protos.NodeGroupTemplateNodeInfoResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got NodeGroupTemplateNodeInfo request")

// 	info, err := c.getNodeGroupTemplateNodeInfo(req.GetId())
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get node info template: %w", err)
// 	}

// 	b, err := info.Marshal()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to marshal node info template: %w", err)
// 	}

// 	return &protos.NodeGroupTemplateNodeInfoResponse{NodeBytes: b}, nil
// }

// // NodeGroupGetOptions returns NodeGroupAutoscalingOptions that should be used for this particular
// // NodeGroup. Returning a grpc error will result in using default options.
// // Implementation optional
// func (c *ClaudieCloudProvider) NodeGroupGetOptions(_ context.Context, req *protos.NodeGroupAutoscalingOptionsRequest) (*protos.NodeGroupAutoscalingOptionsResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got NodeGroupGetOptions request")

// 	return &protos.NodeGroupAutoscalingOptionsResponse{NodeGroupAutoscalingOptions: req.GetDefaults()}, nil
// }

// // getNodeGroupTemplateNodeInfo returns a template for the new node from specified node group.
// func (c *ClaudieCloudProvider) getNodeGroupTemplateNodeInfo(nodeGroupId string) (*k8sV1.Node, error) {
// 	if ngc, ok := c.managedPools[nodeGroupId]; ok {
// 		np := ngc.Immutable.Nodepool

// 		node := k8sV1.Node{
// 			Status: k8sV1.NodeStatus{
// 				Conditions: buildReadyConditions(),
// 			},
// 		}

// 		l, err := nodes.GetAllLabels(np, c.nodeManager, nil)
// 		if err != nil {
// 			return nil, err
// 		}
// 		node.Labels = l
// 		node.Spec.Taints = nodes.GetAllTaints(np, nil)
// 		node.Status.Capacity = c.nodeManager.GetCapacity(np.GetDynamicNodePool())
// 		node.Status.Allocatable = node.Status.Capacity
// 		node.Spec.ProviderID = fmt.Sprintf(nodes.ProviderIdFormat, fmt.Sprintf("%s-N", np.Name))
// 		return &node, nil
// 	}

// 	return nil, nil
// }

// // buildReadyConditions returns default ready condition for the node.
// func buildReadyConditions() []k8sV1.NodeCondition {
// 	lastTransition := time.Now().Add(-time.Minute)
// 	return []k8sV1.NodeCondition{
// 		{
// 			Type:               k8sV1.NodeReady,
// 			Status:             k8sV1.ConditionTrue,
// 			LastTransitionTime: metaV1.Time{Time: lastTransition},
// 		},
// 		{
// 			Type:               k8sV1.NodeNetworkUnavailable,
// 			Status:             k8sV1.ConditionFalse,
// 			LastTransitionTime: metaV1.Time{Time: lastTransition},
// 		},
// 		{
// 			Type:               k8sV1.NodeDiskPressure,
// 			Status:             k8sV1.ConditionFalse,
// 			LastTransitionTime: metaV1.Time{Time: lastTransition},
// 		},
// 		{
// 			Type:               k8sV1.NodeMemoryPressure,
// 			Status:             k8sV1.ConditionFalse,
// 			LastTransitionTime: metaV1.Time{Time: lastTransition},
// 		},
// 	}
// }
