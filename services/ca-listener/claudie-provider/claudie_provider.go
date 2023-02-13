package claudie_provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/Berops/claudie/proto/pb"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

type ClaudieCloudProvider struct {
	protos.UnimplementedCloudProviderServer

	claudieClient *pb.ContextBoxServiceClient
	cluster       *pb.K8Scluster
}

func NewClaudieCloudProvider(projectName, clusterName *string) *ClaudieCloudProvider {
	// Connect to Claudie and retrieve *pb.K8Scluster
	return &ClaudieCloudProvider{}
}

var ErrNotImplemented = errors.New("Not Implemented")

// NodeGroups returns all node groups configured for this cloud provider.
func (c *ClaudieCloudProvider) NodeGroups(_ context.Context, req *protos.NodeGroupsRequest) (*protos.NodeGroupsResponse, error) {
	nodeGroups := make([]*protos.NodeGroup, 0, len(c.cluster.ClusterInfo.NodePools))
	for _, nodepool := range c.cluster.ClusterInfo.NodePools {
		if nodepool.AutoscalerConfig != nil {
			ng := &protos.NodeGroup{
				Id:      fmt.Sprintf("%s-%s", c.cluster.ClusterInfo.Name, nodepool.Name),
				MinSize: nodepool.AutoscalerConfig.Min,
				MaxSize: nodepool.AutoscalerConfig.Max,
				Debug:   fmt.Sprintf("Nodepool %s of cluster %s", nodepool.Name, c.cluster.ClusterInfo.Name),
			}
			nodeGroups = append(nodeGroups, ng)
		}
	}
	return &protos.NodeGroupsResponse{NodeGroups: nodeGroups}, nil
}

// NodeGroupForNode returns the node group for the given node.
// The node group id is an empty string if the node should not
// be processed by cluster autoscaler.
func (c *ClaudieCloudProvider) NodeGroupForNode(_ context.Context, req *protos.NodeGroupForNodeRequest) (*protos.NodeGroupForNodeResponse, error) {
	return &protos.NodeGroupForNodeResponse{NodeGroup: nil}, nil
}

// PricingNodePrice returns a theoretical minimum price of running a node for
// a given period of time on a perfectly matching machine.
// Implementation optional.
func (c *ClaudieCloudProvider) PricingNodePrice(_ context.Context, req *protos.PricingNodePriceRequest) (*protos.PricingNodePriceResponse, error) {
	return nil, ErrNotImplemented
}

// PricingPodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
// Implementation optional.
func (c *ClaudieCloudProvider) PricingPodPrice(_ context.Context, req *protos.PricingPodPriceRequest) (*protos.PricingPodPriceResponse, error) {
	return nil, ErrNotImplemented
}

// GPULabel returns the label added to nodes with GPU resource.
func (c *ClaudieCloudProvider) GPULabel(_ context.Context, req *protos.GPULabelRequest) (*protos.GPULabelResponse, error) {
	return &protos.GPULabelResponse{Label: "claudie.io/gpu-node"}, nil
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (c *ClaudieCloudProvider) GetAvailableGPUTypes(_ context.Context, req *protos.GetAvailableGPUTypesRequest) (*protos.GetAvailableGPUTypesResponse, error) {
	return &protos.GetAvailableGPUTypesResponse{}, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (c *ClaudieCloudProvider) Cleanup(_ context.Context, req *protos.CleanupRequest) (*protos.CleanupResponse, error) {
	return &protos.CleanupResponse{}, c.cleanUp()
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *ClaudieCloudProvider) Refresh(_ context.Context, req *protos.RefreshRequest) (*protos.RefreshResponse, error) {
	return &protos.RefreshResponse{}, c.refresh()
}
