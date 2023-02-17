package claudie_provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

// NodeGroupTargetSize returns the current target size of the node group. It is possible
// that the number of nodes in Kubernetes is different at the moment but should be equal
// to the size of a node group once everything stabilizes (new nodes finish startup and
// registration or removed nodes are deleted completely).
func (c *ClaudieCloudProvider) NodeGroupTargetSize(_ context.Context, req *protos.NodeGroupTargetSizeRequest) (*protos.NodeGroupTargetSizeResponse, error) {
	log.Info().Msgf("Got NodeGroupTargetSize request")
	if size, ok := c.nodeGroupTargetSizeCache[req.GetId()]; ok {
		log.Debug().Msgf("Returning target size %d for nodepool %s", size, req.GetId())
		return &protos.NodeGroupTargetSizeResponse{TargetSize: size}, nil
	}
	return nil, fmt.Errorf("nodeGroup %s was not found", req.Id)
}

// NodeGroupIncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use NodeGroupDeleteNodes. This function should wait until
// node group size is updated.
func (c *ClaudieCloudProvider) NodeGroupIncreaseSize(_ context.Context, req *protos.NodeGroupIncreaseSizeRequest) (*protos.NodeGroupIncreaseSizeResponse, error) {
	log.Info().Msgf("Got NodeGroupIncreaseSize request for nodepool %s by %d", req.GetId(), req.GetDelta())
	for _, nodepool := range c.configCluster.ClusterInfo.NodePools {
		// Find the nodepool.
		if nodepool.Name == req.GetId() {
			// Check & update the new Count.
			newCount := nodepool.Count + req.GetDelta()
			if newCount > nodepool.AutoscalerConfig.Max {
				return nil, fmt.Errorf("could not add new nodes, as that would be larger than max size of the nodepool; current size %d, requested delta %d", nodepool.Count, req.GetDelta())
			}
			nodepool.Count = newCount
			// Update nodepool in Claudie.
			if err := c.updateNodepool(nodepool); err != nil {
				return nil, fmt.Errorf("failed to update nodepool %s : %w", nodepool.Name, err)
			}
			return &protos.NodeGroupIncreaseSizeResponse{}, nil
		}
	}
	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
}

// NodeGroupDeleteNodes deletes nodes from this node group (and also decreasing the size
// of the node group with that). Error is returned either on failure or if the given node
// doesn't belong to this node group. This function should wait until node group size is updated.
func (c *ClaudieCloudProvider) NodeGroupDeleteNodes(_ context.Context, req *protos.NodeGroupDeleteNodesRequest) (*protos.NodeGroupDeleteNodesResponse, error) {
	log.Info().Msgf("Got NodeGroupDeleteNodes request for nodepool %s", req.GetId())
	for _, nodepool := range c.configCluster.ClusterInfo.NodePools {
		// Find the nodepool.
		if nodepool.Name == req.GetId() {
			// Check & update the new Count.
			newCount := nodepool.Count - int32(len(req.GetNodes()))
			if newCount < nodepool.AutoscalerConfig.Min {
				return nil, fmt.Errorf("could not remove nodes, as that would be smaller than min size of the nodepool; current size %d, requested removal %d", nodepool.Count, len(req.GetNodes()))
			}
			nodepool.Count = newCount
			// Update nodes slice
			deleteNodes := make([]*pb.Node, 0, len(req.Nodes))
			remainNodes := make([]*pb.Node, 0, len(nodepool.Nodes)-len(req.Nodes))
			for i, node := range nodepool.Nodes {
				nodeId := fmt.Sprintf("%s-%d", nodepool.Name, i+1)
				if containId(req.GetNodes(), nodeId) {
					deleteNodes = append(deleteNodes, node)
				} else {
					remainNodes = append(remainNodes, node)
				}
			}
			// Reorder node, since they are deleted from the end
			nodepool.Nodes = append(remainNodes, deleteNodes...)
			// Update nodepool in Claudie.
			if err := c.updateNodepool(nodepool); err != nil {
				return nil, fmt.Errorf("failed to update nodepool %s : %w", nodepool.Name, err)
			}
			return &protos.NodeGroupDeleteNodesResponse{}, nil
		}
	}
	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
}

// NodeGroupDecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the request
// for new nodes that have not been yet fulfilled. Delta should be negative. It is assumed
// that cloud provider will not delete the existing nodes if the size when there is an option
// to just decrease the target.
func (c *ClaudieCloudProvider) NodeGroupDecreaseTargetSize(_ context.Context, req *protos.NodeGroupDecreaseTargetSizeRequest) (*protos.NodeGroupDecreaseTargetSizeResponse, error) {
	log.Info().Msgf("Got NodeGroupDecreaseTargetSize request")
	if size, ok := c.nodeGroupTargetSizeCache[req.GetId()]; ok {
		newSize := size + req.GetDelta()
		if newSize >= 0 {
			c.nodeGroupTargetSizeCache[req.GetId()] = newSize
			return &protos.NodeGroupDecreaseTargetSizeResponse{}, nil
		} else {
			return nil, fmt.Errorf("nodeGroup %s can not decrease target size bellow zero; current target size %d, requested delta %d", req.Id, size, req.Delta)
		}
	}
	return nil, fmt.Errorf("nodeGroup %s was not found", req.Id)
}

// NodeGroupNodes returns a list of all nodes that belong to this node group.
func (c *ClaudieCloudProvider) NodeGroupNodes(_ context.Context, req *protos.NodeGroupNodesRequest) (*protos.NodeGroupNodesResponse, error) {
	log.Info().Msgf("Got NodeGroupNodes request")
	nodes := make([]*protos.Instance, 0)
	for _, np := range c.configCluster.ClusterInfo.NodePools {
		if np.Name == req.GetId() {
			for i := range np.Nodes {
				instance := &protos.Instance{
					Id: fmt.Sprintf("%s-%d", np.Name, i+1),
					Status: &protos.InstanceStatus{
						// If there is an instance in the config.K8Scluster, then it is always considered running.
						InstanceState: protos.InstanceStatus_instanceRunning,
					},
				}
				nodes = append(nodes, instance)
			}
		}
	}
	log.Debug().Msgf("NodeGroupForNodes returns %v", nodes)
	return &protos.NodeGroupNodesResponse{Instances: nodes}, nil
}

// NodeGroupTemplateNodeInfo returns a structure of an empty (as if just started) node,
// with all of the labels, capacity and allocatable information. This will be used in
// scale-up simulations to predict what would a new node look like if a node group was expanded.
// Implementation optional.
func (c *ClaudieCloudProvider) NodeGroupTemplateNodeInfo(_ context.Context, req *protos.NodeGroupTemplateNodeInfoRequest) (*protos.NodeGroupTemplateNodeInfoResponse, error) {
	log.Info().Msgf("Got NodeGroupTemplateNodeInfo request")
	return nil, ErrNotImplemented
}

// GetOptions returns NodeGroupAutoscalingOptions that should be used for this particular
// NodeGroup. Returning a grpc error will result in using default options.
// Implementation optional
func (c *ClaudieCloudProvider) NodeGroupGetOptions(_ context.Context, req *protos.NodeGroupAutoscalingOptionsRequest) (*protos.NodeGroupAutoscalingOptionsResponse, error) {
	log.Info().Msgf("Got NodeGroupGetOptions request")
	return &protos.NodeGroupAutoscalingOptionsResponse{NodeGroupAutoscalingOptions: req.GetDefaults()}, nil
}

func (c *ClaudieCloudProvider) updateNodepool(nodepool *pb.NodePool) error {
	// Update the nodepool in the Claudie.
	var cc *grpc.ClientConn
	var err error

	cboxURL := strings.ReplaceAll(envs.ContextBoxURL, ":tcp://", "")
	if cc, err = utils.GrpcDialWithInsecure("context-box", cboxURL); err != nil {
		return fmt.Errorf("failed to dial context-box at %s : %w", envs.ContextBoxURL, err)
	}
	cbox := pb.NewContextBoxServiceClient(cc)
	if _, err := cbox.UpdateNodepool(context.Background(),
		&pb.UpdateNodepoolRequest{
			ProjectName: c.projectName,
			ClusterName: c.configCluster.ClusterInfo.Name,
			Nodepool:    nodepool,
		}); err != nil {
		return fmt.Errorf("error while updating the state in the Claudie : %w", err)
	}
	return nil
}

func containId(nodes []*protos.ExternalGrpcNode, nodeId string) bool {
	for _, node := range nodes {
		if node.Name == nodeId {
			return true
		}
	}
	return false
}
