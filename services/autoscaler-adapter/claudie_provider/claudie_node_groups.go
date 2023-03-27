package claudie_provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/nodes"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

// NodeGroupTargetSize returns the current target size of the node group. It is possible
// that the number of nodes in Kubernetes is different at the moment but should be equal
// to the size of a node group once everything stabilizes (new nodes finish startup and
// registration or removed nodes are deleted completely).
func (c *ClaudieCloudProvider) NodeGroupTargetSize(_ context.Context, req *protos.NodeGroupTargetSizeRequest) (*protos.NodeGroupTargetSizeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupTargetSize request")
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		log.Debug().Msgf("Returning target size %d for nodepool %s", ngc.targetSize, req.GetId())
		return &protos.NodeGroupTargetSizeResponse{TargetSize: ngc.targetSize}, nil
	}
	return nil, fmt.Errorf("nodeGroup %s was not found", req.Id)
}

// NodeGroupIncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use NodeGroupDeleteNodes. This function should wait until
// node group size is updated.
func (c *ClaudieCloudProvider) NodeGroupIncreaseSize(_ context.Context, req *protos.NodeGroupIncreaseSizeRequest) (*protos.NodeGroupIncreaseSizeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupIncreaseSize request for nodepool %s by %d", req.GetId(), req.GetDelta())
	// Find the nodepool.
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		// Check & update the new Count.
		newCount := ngc.nodepool.Count + req.GetDelta()
		if newCount > ngc.nodepool.AutoscalerConfig.Max {
			return nil, fmt.Errorf("could not add new nodes, as that would be larger than max size of the nodepool; current size %d, requested delta %d", ngc.nodepool.Count, req.GetDelta())
		}
		ngc.nodepool.Count = newCount
		ngc.targetSize = newCount
		// Update nodepool in Claudie.
		if err := c.updateNodepool(ngc.nodepool); err != nil {
			return nil, fmt.Errorf("failed to update nodepool %s : %w", ngc.nodepool.Name, err)
		}
		return &protos.NodeGroupIncreaseSizeResponse{}, nil
	}

	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
}

// NodeGroupDeleteNodes deletes nodes from this node group (and also decreasing the size
// of the node group with that). Error is returned either on failure or if the given node
// doesn't belong to this node group. This function should wait until node group size is updated.
func (c *ClaudieCloudProvider) NodeGroupDeleteNodes(_ context.Context, req *protos.NodeGroupDeleteNodesRequest) (*protos.NodeGroupDeleteNodesResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupDeleteNodes request for nodepool %s", req.GetId())
	// Find the nodepool.
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		// Check & update the new Count.
		newCount := ngc.nodepool.Count - int32(len(req.GetNodes()))
		if newCount < ngc.nodepool.AutoscalerConfig.Min {
			return nil, fmt.Errorf("could not remove nodes, as that would be smaller than min size of the nodepool; current size %d, requested removal %d", ngc.nodepool.Count, len(req.GetNodes()))
		}
		ngc.nodepool.Count = newCount
		ngc.targetSize = newCount
		// Update nodes slice
		deleteNodes := make([]*pb.Node, 0, len(req.Nodes))
		remainNodes := make([]*pb.Node, 0, len(ngc.nodepool.Nodes)-len(req.Nodes))
		for _, node := range ngc.nodepool.Nodes {
			nodeId := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-%s-", c.configCluster.ClusterInfo.Name, c.configCluster.ClusterInfo.Hash))
			if containsId(req.GetNodes(), nodeId) {
				log.Debug().Msgf("Adding node %s to delete nodes slice", nodeId)
				deleteNodes = append(deleteNodes, node)
			} else {
				remainNodes = append(remainNodes, node)
			}
		}
		// Reorder node, since they are deleted from the end
		ngc.nodepool.Nodes = append(remainNodes, deleteNodes...)
		// Update nodepool in Claudie.
		if err := c.updateNodepool(ngc.nodepool); err != nil {
			return nil, fmt.Errorf("failed to update nodepool %s : %w", ngc.nodepool.Name, err)
		}
		return &protos.NodeGroupDeleteNodesResponse{}, nil
	}
	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
}

// NodeGroupDecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the request
// for new nodes that have not been yet fulfilled. Delta should be negative. It is assumed
// that cloud provider will not delete the existing nodes if the size when there is an option
// to just decrease the target.
func (c *ClaudieCloudProvider) NodeGroupDecreaseTargetSize(_ context.Context, req *protos.NodeGroupDecreaseTargetSizeRequest) (*protos.NodeGroupDecreaseTargetSizeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupDecreaseTargetSize request")
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		newSize := ngc.targetSize + req.GetDelta()
		if newSize >= 0 {
			c.nodesCache[req.GetId()].targetSize = newSize
			return &protos.NodeGroupDecreaseTargetSizeResponse{}, nil
		}
		return nil, fmt.Errorf("nodeGroup %s can not decrease target size bellow zero; current target size %d, requested delta %d", req.Id, ngc.targetSize, req.Delta)
	}
	return nil, fmt.Errorf("nodeGroup %s was not found", req.Id)
}

// NodeGroupNodes returns a list of all nodes that belong to this node group.
func (c *ClaudieCloudProvider) NodeGroupNodes(_ context.Context, req *protos.NodeGroupNodesRequest) (*protos.NodeGroupNodesResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupNodes request")
	instances := make([]*protos.Instance, 0)
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		for _, node := range ngc.nodepool.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-%s-", c.configCluster.ClusterInfo.Name, c.configCluster.ClusterInfo.Hash))
			instance := &protos.Instance{
				Id: fmt.Sprintf(nodes.ProviderIdFormat, nodeName),
				Status: &protos.InstanceStatus{
					// If there is an instance in the config.K8Scluster, then it is always considered running.
					InstanceState: protos.InstanceStatus_instanceRunning,
				},
			}
			instances = append(instances, instance)
		}
	}
	log.Debug().Msgf("NodeGroupForNodes returns %v", instances)
	return &protos.NodeGroupNodesResponse{Instances: instances}, nil
}

// NodeGroupTemplateNodeInfo returns a structure of an empty (as if just started) node,
// with all of the labels, capacity and allocatable information. This will be used in
// scale-up simulations to predict what would a new node look like if a node group was expanded.
// Implementation optional.
func (c *ClaudieCloudProvider) NodeGroupTemplateNodeInfo(_ context.Context, req *protos.NodeGroupTemplateNodeInfoRequest) (*protos.NodeGroupTemplateNodeInfoResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupTemplateNodeInfo request")
	info := c.getNodeGroupTemplateNodeInfo(req.GetId())
	return &protos.NodeGroupTemplateNodeInfoResponse{NodeInfo: info}, nil
}

// NodeGroupGetOptions returns NodeGroupAutoscalingOptions that should be used for this particular
// NodeGroup. Returning a grpc error will result in using default options.
// Implementation optional
func (c *ClaudieCloudProvider) NodeGroupGetOptions(_ context.Context, req *protos.NodeGroupAutoscalingOptionsRequest) (*protos.NodeGroupAutoscalingOptionsResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupGetOptions request")
	return &protos.NodeGroupAutoscalingOptionsResponse{NodeGroupAutoscalingOptions: req.GetDefaults()}, nil
}

// updateNodepool will call context-box UpdateNodepool method to save any changes to the database. This will also initiate build of the changed nodepool.
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

// containsId checks if nodes in specified slice contain specific id.
func containsId(nodes []*protos.ExternalGrpcNode, nodeId string) bool {
	for _, node := range nodes {
		if node.Name == nodeId {
			return true
		}
	}
	return false
}
