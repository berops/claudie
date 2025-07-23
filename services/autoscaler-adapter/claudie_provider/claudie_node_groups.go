package claudie_provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/server/domain/utils/nodes"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
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
		log.Debug().Str("nodepool", req.GetId()).Msgf("Returning target size %d for nodepool", ngc.targetSize)
		return &protos.NodeGroupTargetSizeResponse{TargetSize: ngc.targetSize}, nil
	}
	return nil, fmt.Errorf("nodeGroup %s was not found", req.Id)
}

// NodeGroupIncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use NodeGroupDeleteNodes. This function should wait until
// node group size is updated.
func (c *ClaudieCloudProvider) NodeGroupIncreaseSize(ctx context.Context, req *protos.NodeGroupIncreaseSizeRequest) (*protos.NodeGroupIncreaseSizeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Str("nodepool", req.GetId()).Msgf("Got NodeGroupIncreaseSize request for nodepool by %d", req.GetDelta())
	// Find the nodepool.
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		// Check & update the new Count.
		newCount := ngc.nodepool.GetDynamicNodePool().Count + req.GetDelta()
		if newCount > ngc.nodepool.GetDynamicNodePool().AutoscalerConfig.Max {
			return nil, fmt.Errorf("could not add new nodes, as that would be larger than max size of the nodepool; current size %d, requested delta %d", ngc.nodepool.GetDynamicNodePool().Count, req.GetDelta())
		}

		temp := proto.Clone(ngc.nodepool).(*spec.NodePool)
		temp.GetDynamicNodePool().Count = newCount

		if err := c.updateNodepool(ctx, temp); err != nil {
			return nil, fmt.Errorf("failed to update nodepool %s : %w", temp.Name, err)
		}
		if err := c.sendAutoscalerEvent(); err != nil {
			return nil, fmt.Errorf("failed to send autoscaler event %s : %w", temp.Name, err)
		}

		ngc.targetSize = newCount
		ngc.nodepool = temp

		return &protos.NodeGroupIncreaseSizeResponse{}, nil
	}

	return nil, fmt.Errorf("could not find the nodepool with id %s", req.GetId())
}

// NodeGroupDeleteNodes deletes nodes from this node group (and also decreasing the size
// of the node group with that). Error is returned either on failure or if the given node
// doesn't belong to this node group. This function should wait until node group size is updated.
func (c *ClaudieCloudProvider) NodeGroupDeleteNodes(ctx context.Context, req *protos.NodeGroupDeleteNodesRequest) (*protos.NodeGroupDeleteNodesResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Str("nodepool", req.GetId()).Msgf("Got NodeGroupDeleteNodes request for nodepool")
	// Find the nodepool.
	if ngc, ok := c.nodesCache[req.GetId()]; ok {
		// Check & update the new Count.
		newCount := ngc.nodepool.GetDynamicNodePool().GetCount() - int32(len(req.GetNodes()))
		if newCount < ngc.nodepool.GetDynamicNodePool().AutoscalerConfig.GetMin() {
			return nil, fmt.Errorf("could not remove nodes, as that would be smaller than min size of the nodepool; current size %d, requested removal %d", ngc.nodepool.GetDynamicNodePool().Count, len(req.GetNodes()))
		}

		temp := proto.Clone(ngc.nodepool).(*spec.NodePool)
		temp.GetDynamicNodePool().Count = newCount

		temp.Nodes = slices.DeleteFunc(temp.Nodes, func(node *spec.Node) bool {
			nodeId := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-%s-", c.configCluster.ClusterInfo.Name, c.configCluster.ClusterInfo.Hash))
			return slices.ContainsFunc(req.GetNodes(), func(n *protos.ExternalGrpcNode) bool { return n.Name == nodeId })
		})

		if err := c.updateNodepool(ctx, temp); err != nil {
			return nil, fmt.Errorf("failed to update nodepool %s : %w", temp.Name, err)
		}
		if err := c.sendAutoscalerEvent(); err != nil {
			return nil, fmt.Errorf("failed to send autoscaler event %s : %w", temp.Name, err)
		}

		ngc.targetSize = newCount
		ngc.nodepool = temp

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
	info, err := c.getNodeGroupTemplateNodeInfo(req.GetId())
	if err != nil {
		return nil, fmt.Errorf("failed to get node info template: %w", err)
	}
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

func (c *ClaudieCloudProvider) updateNodepool(ctx context.Context, nodepool *spec.NodePool) error {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return fmt.Errorf("failed to connect to manager: %w", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			log.Err(err).Msgf("Failed to close manager connection")
		}
	}()

	resp, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: c.projectName})
	if err != nil {
		if errors.Is(err, managerclient.ErrNotFound) {
			log.Warn().Msgf("%v", err)
		}
		return fmt.Errorf("failed to get config %s : %w", c.projectName, err)
	}

	// Prevent autoscaling request when the InputManifest is in ERROR.
	for cluster, state := range resp.Config.Clusters {
		if cluster != c.configCluster.ClusterInfo.Name {
			continue
		}
		if state.State.Status == spec.Workflow_ERROR {
			log.Error().Msgf("Failed to send autoscaling request. Cluster %s is in ERROR", cluster)
			// Error return is necessary in order to prevent to call sendAutoscalerEvent.
			return fmt.Errorf("failed to send autoscaling request. Cluster: %s is in ERROR", cluster)
		}
	}

	description := fmt.Sprintf("UpdateNodePoolRequest cluster %q config %q", c.configCluster.ClusterInfo.Name, resp.Config.Name)

	err = managerclient.Retry(&log.Logger, description, func() error {
		err := manager.UpdateNodePool(ctx, &managerclient.UpdateNodePoolRequest{
			Config:   c.projectName,
			Cluster:  c.configCluster.ClusterInfo.Name,
			NodePool: nodepool,
		})
		if errors.Is(err, managerclient.ErrNotFound) {
			log.Warn().Msgf("can't update nodepool %q cluster %q config %q: %v", nodepool.Name, c.configCluster.ClusterInfo.Name, c.projectName, err)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("error while updating the state in the Claudie : %w", err)
	}
	return nil
}
