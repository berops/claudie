package claudie_provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/managerv2/client"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	k8sV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

const (
	// Default GPU label.
	GpuLabel = "claudie.io/gpu-node"
)

type NodeGroup struct {
	// Cluster Autoscaler NodeGroup representation.
	G *protos.NodeGroup

	// Cached NodePool from claudie.
	N *spec.NodePool

	// Cached target size of the nodepool.
	TargetSize int32
}

// NodeGroups returns all node groups configured for this cloud provider.
func (m *Manager) NodeGroups(_ context.Context, req *protos.NodeGroupsRequest) (*protos.NodeGroupsResponse, error) {
	log.Info().Msg("Handling NodeGroups request")

	m.lock.Lock()
	defer m.lock.Unlock()

	groups := make([]*protos.NodeGroup, 0, len(m.Groups))
	for _, group := range m.Groups {
		groups = append(groups, &protos.NodeGroup{
			Id:      group.G.Id,
			MinSize: group.G.MinSize,
			MaxSize: group.G.MaxSize,
			Debug:   group.G.Debug,
		})
	}

	return &protos.NodeGroupsResponse{NodeGroups: groups}, nil
}

// NodeGroupForNode returns the node group for the given node.
// The node group id is an empty string if the node should not
// be processed by cluster autoscaler.
func (m *Manager) NodeGroupForNode(_ context.Context, req *protos.NodeGroupForNodeRequest) (*protos.NodeGroupForNodeResponse, error) {
	log.Info().Msg("Handling NodeGroupForNode request")

	nodeName := req.Node.Name

	m.lock.Lock()
	defer m.lock.Unlock()

	// Initialize as empty response.
	nodeGroup := &protos.NodeGroup{}

	// Try to find if node is from any NodeGroup
	for id, group := range m.Groups {
		// If node name contains ng.Id (nodepool name), return this NodeGroup
		if strings.Contains(nodeName, id) {
			nodeGroup = &protos.NodeGroup{
				Id:      group.G.Id,
				MinSize: group.G.MinSize,
				MaxSize: group.G.MaxSize,
				Debug:   group.G.Debug,
			}

			break
		}
	}

	return &protos.NodeGroupForNodeResponse{NodeGroup: nodeGroup}, nil
}

// PricingNodePrice returns a theoretical minimum price of running a node for
// a given period of time on a perfectly matching machine.
// Implementation optional.
func (m *Manager) PricingNodePrice(_ context.Context, _ *protos.PricingNodePriceRequest) (*protos.PricingNodePriceResponse, error) {
	log.Info().Msg("Handling PricingNodePrice request")
	return nil, status.Error(codes.Unimplemented, "Pricing unimplemented")
}

// PricingPodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
// Implementation optional.
func (m *Manager) PricingPodPrice(_ context.Context, _ *protos.PricingPodPriceRequest) (*protos.PricingPodPriceResponse, error) {
	log.Info().Msg("Handling PricingPodPrice request")
	return nil, status.Error(codes.Unimplemented, "PricingPodPrice unimplemented")
}

// GPULabel returns the label added to nodes with GPU resource.
func (m *Manager) GPULabel(_ context.Context, _ *protos.GPULabelRequest) (*protos.GPULabelResponse, error) {
	log.Info().Msgf("Handling GPULabel request")
	return &protos.GPULabelResponse{Label: GpuLabel}, nil
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (m *Manager) GetAvailableGPUTypes(_ context.Context, _ *protos.GetAvailableGPUTypesRequest) (*protos.GetAvailableGPUTypesResponse, error) {
	log.Info().Msgf("Handling GetAvailableGPUTypes request")
	return &protos.GetAvailableGPUTypesResponse{}, nil
}

// NodeGroupTargetSize returns the current target size of the node group. It is possible
// that the number of nodes in Kubernetes is different at the moment but should be equal
// to the size of a node group once everything stabilizes (new nodes finish startup and
// registration or removed nodes are deleted completely).
func (m *Manager) NodeGroupTargetSize(_ context.Context, req *protos.NodeGroupTargetSizeRequest) (*protos.NodeGroupTargetSizeResponse, error) {
	log.Info().Msgf("Handling NodeGroupTargetSize request")

	m.lock.Lock()
	defer m.lock.Unlock()

	g, ok := m.Groups[req.Id]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "nodeGroup %q was not found", req.Id)
	}

	log.
		Debug().
		Str("nodepool", req.Id).
		Msgf("Returning target size %d for nodepool", g.TargetSize)

	return &protos.NodeGroupTargetSizeResponse{TargetSize: g.TargetSize}, nil
}

// NodeGroupIncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use NodeGroupDeleteNodes. This function should wait until
// node group size is updated.
func (m *Manager) NodeGroupIncreaseSize(
	ctx context.Context,
	req *protos.NodeGroupIncreaseSizeRequest,
) (*protos.NodeGroupIncreaseSizeResponse, error) {
	log.
		Info().
		Str("nodepool", req.Id).
		Msgf("Handling NodeGroupIncreaseSize request for nodepool by %d", req.Delta)

	m.lock.Lock()
	defer m.lock.Unlock()

	g, ok := m.Groups[req.Id]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "nodepool with id %q not found", req.Id)
	}

	newDesiredTargetSize := g.TargetSize + req.Delta
	if newDesiredTargetSize > g.G.MaxSize {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"new desired target size %d would overflow node group max %d",
			newDesiredTargetSize,
			g.G.MaxSize,
		)
	}

	resp, err := UpdateNodePoolTargetSize(ctx, managerclient.NodePoolUpdateTargetSizeRequest{
		Config:     m.Immutable.Config,
		Cluster:    m.Immutable.ClusterName,
		NodePool:   req.Id,
		TargetSize: newDesiredTargetSize,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update target size of nodepool %q: %v", req.Id, err)
	}
	if newDesiredTargetSize != resp.TargetSize {
		return nil, status.Errorf(codes.Internal, "failed to increase target size for nodepool %q", req.Id)
	}

	g.TargetSize = resp.TargetSize

	if err := sendAutoscalerEvent(m.K8sCtx); err != nil {
		log.Err(err).Msg("Failed to notify operator for autoscaled event")
	}

	return &protos.NodeGroupIncreaseSizeResponse{}, nil
}

// NodeGroupDeleteNodes deletes nodes from this node group (and also decreasing the size
// of the node group with that). Error is returned either on failure or if the given node
// doesn't belong to this node group. This function should wait until node group size is updated.
func (m *Manager) NodeGroupDeleteNodes(ctx context.Context, req *protos.NodeGroupDeleteNodesRequest) (*protos.NodeGroupDeleteNodesResponse, error) {
	log.
		Info().
		Str("nodepool", req.Id).
		Msg("Handling NodeGroupDeleteNodes request for nodepool")

	m.lock.Lock()
	defer m.lock.Unlock()

	g, ok := m.Groups[req.Id]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "nodepool with id %q not found", req.Id)
	}

	for _, n := range req.Nodes {
		nodeName := fmt.Sprintf("%s-%s-%s", m.Immutable.ClusterName, m.Immutable.ClusterHash, n.Name)
		decrement := true

		resp, err := MarkNodeForDeletion(ctx, managerclient.MarkNodeForDeletionRequest{
			Config:                         m.Immutable.Config,
			Cluster:                        m.Immutable.ClusterName,
			NodePool:                       req.Id,
			Node:                           nodeName,
			ShouldDecrementDesiredCapacity: &decrement,
		})
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"failed to mark node %q from nodepool %q for deletion: %v", n.Name, req.Id, err,
			)
		}
		if resp.TargetSize != (int64(g.TargetSize) - 1) {
			return nil, status.Errorf(
				codes.Internal,
				"failed to mark node %q from nodepool %q for deletion", n.Name, req.Id,
			)
		}

		// decrement target size by one.
		g.TargetSize -= 1
	}

	if err := sendAutoscalerEvent(m.K8sCtx); err != nil {
		log.Err(err).Msg("Failed to notify operator for autoscale event")
	}

	return &protos.NodeGroupDeleteNodesResponse{}, nil
}

// NodeGroupDecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the request
// for new nodes that have not been yet fulfilled. Delta should be negative. It is assumed
// that cloud provider will not delete the existing nodes if the size when there is an option
// to just decrease the target.
func (m *Manager) NodeGroupDecreaseTargetSize(_ context.Context, req *protos.NodeGroupDecreaseTargetSizeRequest) (*protos.NodeGroupDecreaseTargetSizeResponse, error) {
	// Requests for new nodes are always fulfilled so we cannot decrease
	// the size of the nodepool without actually going through the deletion
	// of the nodes within a nodepool.
	//
	// Note(despire): Seems that this function is not mandatory to implement
	// looking at the implementations in:
	// https://github.com/search?q=repo%3Akubernetes%2Fautoscaler+DecreaseTargetSize&type=code
	//
	// Some providers avoid implementing it (Azure, for example) when they do not support
	// decreasing the targetsize without actual deletion.
	return new(protos.NodeGroupDecreaseTargetSizeResponse), nil
}

// NodeGroupNodes returns a list of all nodes that belong to this node group.
func (m *Manager) NodeGroupNodes(_ context.Context, req *protos.NodeGroupNodesRequest) (*protos.NodeGroupNodesResponse, error) {
	log.Info().Msgf("Handling NodeGroupNodes request")

	m.lock.Lock()
	defer m.lock.Unlock()

	instances := make([]*protos.Instance, 0)

	g, ok := m.Groups[req.Id]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "nodepool with id %q not found", req.Id)
	}

	for _, node := range g.N.Nodes {
		nodeName := strings.TrimPrefix(
			node.Name,
			fmt.Sprintf("%s-%s-", m.Immutable.ClusterName, m.Immutable.ClusterHash),
		)

		instances = append(instances, &protos.Instance{
			Id: fmt.Sprintf(nodes.ProviderIdFormat, nodeName),
			// Status: is optional, some providers like AWS or Azure do not even set it.
		})
	}

	log.Debug().Msgf("NodeGroupForNodes returns %v", instances)
	return &protos.NodeGroupNodesResponse{Instances: instances}, nil
}

// NodeGroupTemplateNodeInfo returns a structure of an empty (as if just started) node,
// with all of the labels, capacity and allocatable information. This will be used in
// scale-up simulations to predict what would a new node look like if a node group was expanded.
// Implementation optional.
func (m *Manager) NodeGroupTemplateNodeInfo(_ context.Context, req *protos.NodeGroupTemplateNodeInfoRequest) (*protos.NodeGroupTemplateNodeInfoResponse, error) {
	log.Info().Msgf("Handling NodeGroupTemplateNodeInfo request")

	m.lock.Lock()
	defer m.lock.Unlock()

	info, err := m.getNodeGroupTemplateNodeInfo(req.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get node info template: %w", err)
	}

	b, err := info.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node info template: %w", err)
	}

	return &protos.NodeGroupTemplateNodeInfoResponse{NodeBytes: b}, nil
}

// NodeGroupGetOptions returns NodeGroupAutoscalingOptions that should be used for this particular
// NodeGroup. Returning a grpc error will result in using default options.
// Implementation optional
func (m *Manager) NodeGroupGetOptions(_ context.Context, req *protos.NodeGroupAutoscalingOptionsRequest) (*protos.NodeGroupAutoscalingOptionsResponse, error) {
	log.Info().Msgf("Handling NodeGroupGetOptions request")

	m.lock.Lock()
	defer m.lock.Unlock()

	return &protos.NodeGroupAutoscalingOptionsResponse{NodeGroupAutoscalingOptions: req.GetDefaults()}, nil
}

func UpdateNodePoolTargetSize(ctx context.Context, req managerclient.NodePoolUpdateTargetSizeRequest) (*managerclient.NodePoolUpdateTargetSizeResponse, error) {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to manager: %w", err)
	}

	defer func() {
		if err := manager.Close(); err != nil {
			log.Err(err).Msgf("Failed to close manager connection")
		}
	}()

	return manager.NodePoolUpdateTargetSize(ctx, &req)
}

func MarkNodeForDeletion(ctx context.Context, req managerclient.MarkNodeForDeletionRequest) (*managerclient.MarkNodeForDeletionResponse, error) {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to manager: %w", err)
	}

	defer func() {
		if err := manager.Close(); err != nil {
			log.Err(err).Msgf("Failed to close manager connection")
		}
	}()

	return manager.MarkNodeForDeletion(ctx, &req)
}

// getNodeGroupTemplateNodeInfo returns a template for the new node from specified node group.
func (m *Manager) getNodeGroupTemplateNodeInfo(nodeGroupId string) (*k8sV1.Node, error) {
	if ngc, ok := m.Groups[nodeGroupId]; ok {
		np := ngc.N

		node := k8sV1.Node{
			Status: k8sV1.NodeStatus{
				Conditions: buildReadyConditions(),
			},
		}

		l, err := nodes.GetAllLabels(np, m.NodeManager, nil)
		if err != nil {
			return nil, err
		}
		node.Labels = l
		node.Spec.Taints = nodes.GetAllTaints(np, nil)
		node.Status.Capacity = m.NodeManager.GetCapacity(np.GetDynamicNodePool())
		node.Status.Allocatable = node.Status.Capacity
		node.Spec.ProviderID = fmt.Sprintf(nodes.ProviderIdFormat, fmt.Sprintf("%s-N", np.Name))
		return &node, nil
	}

	return nil, nil
}

// buildReadyConditions returns default ready condition for the node.
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
