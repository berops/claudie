package claudie_provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/autoscaler-adapter/node_manager"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

const (
	GpuLabel = "claudie.io/gpu-node"
)

var (
	ErrNotImplemented = errors.New("Not Implemented")
)

type nodeGroupCache struct {
	nodeGroup  *protos.NodeGroup
	nodepool   *pb.NodePool
	targetSize int32
}

type ClaudieCloudProvider struct {
	protos.UnimplementedCloudProviderServer

	// Name of the Claudie config.
	projectName string
	// Cluster as described in Claudie config.
	configCluster *pb.K8Scluster
	// Map of cached info regarding nodepools
	nodeGroupCache map[string]*nodeGroupCache
	// Node manager
	nodeManager *node_manager.NodeManager
}

func NewClaudieCloudProvider(projectName, clusterName string) *ClaudieCloudProvider {
	// Connect to Claudie and retrieve *pb.K8Scluster
	var cluster *pb.K8Scluster
	var err error
	if cluster, err = getClaudieState(projectName, clusterName); err != nil {
		panic(fmt.Sprintf("Error while getting cluster %s : %v", clusterName, err))
	}
	// Initialise all other variables.
	return &ClaudieCloudProvider{
		projectName:    projectName,
		configCluster:  cluster,
		nodeGroupCache: getNodeGroupCache(cluster.ClusterInfo.NodePools),
	}
}

// getClaudieState returns a *pb.K8Scluster from Claudie, for this particular instance.
func getClaudieState(projectName, clusterName string) (*pb.K8Scluster, error) {
	var cc *grpc.ClientConn
	var err error
	var res *pb.GetConfigFromDBResponse
	cboxURL := strings.ReplaceAll(envs.ContextBoxURL, ":tcp://", "")

	if cc, err = utils.GrpcDialWithInsecure("context-box", cboxURL); err != nil {
		return nil, fmt.Errorf("Failed to dial context-box at %s : %w", cboxURL, err)
	}
	c := pb.NewContextBoxServiceClient(cc)
	if res, err = c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: projectName, Type: pb.IdType_NAME}); err != nil {
		return nil, fmt.Errorf("Failed to get config for project %s : %w", projectName, err)
	}

	for _, cluster := range res.Config.DesiredState.Clusters {
		if cluster.ClusterInfo.Name == clusterName {
			return cluster, nil
		}
	}
	return nil, fmt.Errorf("Failed to find cluster %s in config for a project %s", clusterName, projectName)
}

// getNodeGroupCache returns a map of nodeGroupCache, regarding all information needed based on the nodepools with autoscaling enabled.
func getNodeGroupCache(nodepools []*pb.NodePool) map[string]*nodeGroupCache {
	var ngc = make(map[string]*nodeGroupCache, len(nodepools))
	for _, np := range nodepools {
		// Find autoscaled nodepool.
		if np.AutoscalerConfig != nil {
			// Create nodeGroup struct.
			ng := &protos.NodeGroup{
				Id:      np.Name,
				MinSize: np.AutoscalerConfig.Min,
				MaxSize: np.AutoscalerConfig.Max,
				Debug:   fmt.Sprintf("Nodepool %s with autoscaler config %v", np.Name, np.AutoscalerConfig),
			}
			// Append ng to the final slice.
			ngc[np.Name] = &nodeGroupCache{nodeGroup: ng, nodepool: np, targetSize: np.Count}
		}
	}
	return ngc
}

// NodeGroups returns all node groups configured for this cloud provider.
func (c *ClaudieCloudProvider) NodeGroups(_ context.Context, req *protos.NodeGroupsRequest) (*protos.NodeGroupsResponse, error) {
	log.Info().Msgf("Got NodeGroups request")
	ngs := make([]*protos.NodeGroup, 0, len(c.nodeGroupCache))
	for _, ngc := range c.nodeGroupCache {
		ngs = append(ngs, ngc.nodeGroup)
	}
	return &protos.NodeGroupsResponse{NodeGroups: ngs}, nil
}

// NodeGroupForNode returns the node group for the given node.
// The node group id is an empty string if the node should not
// be processed by cluster autoscaler.
func (c *ClaudieCloudProvider) NodeGroupForNode(_ context.Context, req *protos.NodeGroupForNodeRequest) (*protos.NodeGroupForNodeResponse, error) {
	log.Info().Msgf("Got NodeGroupForNode request")
	nodeName := req.Node.Name
	// Initialise as empty response.
	nodeGroup := &protos.NodeGroup{}
	// Try to find if node is from any NodeGroup
	for id, ngc := range c.nodeGroupCache {
		// If node name contains ng.Id (nodepool name), return this NodeGroup.
		if strings.Contains(nodeName, id) {
			nodeGroup = ngc.nodeGroup
			break
		}
	}
	return &protos.NodeGroupForNodeResponse{NodeGroup: nodeGroup}, nil
}

// PricingNodePrice returns a theoretical minimum price of running a node for
// a given period of time on a perfectly matching machine.
// Implementation optional.
func (c *ClaudieCloudProvider) PricingNodePrice(_ context.Context, req *protos.PricingNodePriceRequest) (*protos.PricingNodePriceResponse, error) {
	log.Info().Msgf("Got PricingNodePrice request; Not implemented")
	return nil, ErrNotImplemented
}

// PricingPodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
// Implementation optional.
func (c *ClaudieCloudProvider) PricingPodPrice(_ context.Context, req *protos.PricingPodPriceRequest) (*protos.PricingPodPriceResponse, error) {
	log.Info().Msgf("Got PricingPodPrice request; Not implemented")
	return nil, ErrNotImplemented
}

// GPULabel returns the label added to nodes with GPU resource.
func (c *ClaudieCloudProvider) GPULabel(_ context.Context, req *protos.GPULabelRequest) (*protos.GPULabelResponse, error) {
	log.Info().Msgf("Got GPULabel request")
	return &protos.GPULabelResponse{Label: GpuLabel}, nil
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (c *ClaudieCloudProvider) GetAvailableGPUTypes(_ context.Context, req *protos.GetAvailableGPUTypesRequest) (*protos.GetAvailableGPUTypesResponse, error) {
	log.Info().Msgf("Got GetAvailableGPUTypes request")
	//TODO
	return &protos.GetAvailableGPUTypesResponse{}, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (c *ClaudieCloudProvider) Cleanup(_ context.Context, req *protos.CleanupRequest) (*protos.CleanupResponse, error) {
	log.Info().Msgf("Got Cleanup request")
	return &protos.CleanupResponse{}, c.cleanup()
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *ClaudieCloudProvider) Refresh(_ context.Context, req *protos.RefreshRequest) (*protos.RefreshResponse, error) {
	log.Info().Msgf("Got Refresh request")
	return &protos.RefreshResponse{}, c.refresh()
}

// cleanup cleans up all the resources claudie provider uses.
func (c *ClaudieCloudProvider) cleanup() error {
	log.Info().Msgf("Cleaning all resources")
	return nil
}

// refresh refreshes the state of the claudie provider based of the state from Claudie.
func (c *ClaudieCloudProvider) refresh() error {
	log.Info().Msgf("Refreshing the state")
	if cluster, err := getClaudieState(c.projectName, c.configCluster.ClusterInfo.Name); err != nil {
		log.Error().Msgf("error while refreshing a state for the cluster %s : %v", c.configCluster.ClusterInfo.Name, err)
		return fmt.Errorf("error while refreshing a state for the cluster %s : %w", c.configCluster.ClusterInfo.Name, err)
	} else {
		c.configCluster = cluster
		c.nodeGroupCache = getNodeGroupCache(cluster.ClusterInfo.NodePools)
		log.Debug().Msgf("Updated state: \n %v ", c.nodeGroupCache)
	}
	return nil
}
