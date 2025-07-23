package claudie_provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/autoscaler-adapter/node_manager"
	cOperator "github.com/berops/claudie/services/claudie-operator/client"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

const (
	// Default GPU label.
	GpuLabel = "claudie.io/gpu-node"
)

var (
	// ErrNotImplemented used for functions which are not implemented.
	ErrNotImplemented = errors.New("not implemented")
	// ErrConfigInDeletion returned when the desired state of the config is nil and cannot perform a refresh operation.
	ErrConfigInDeletion = errors.New("config is marked for deletion")
)

type nodeCache struct {
	// Nodegroup as per Cluster Autoscaler definition.
	nodeGroup *protos.NodeGroup
	// Nodepool as per Claudie definition.
	nodepool *spec.NodePool
	// Target size of node group.
	targetSize int32
}

type ClaudieCloudProvider struct {
	protos.UnimplementedCloudProviderServer

	// Name of the Claudie config.
	projectName string
	// Kubernetes InputManifest resource name
	resourceName string
	// Kubernetes InputManifest resource namespace
	resourceNamespace string
	// Cluster as described in Claudie config.
	configCluster *spec.K8Scluster
	// Map of cached info regarding nodes.
	nodesCache map[string]*nodeCache
	// Node manager.
	nodeManager *node_manager.NodeManager
	// Server mutex
	lock sync.Mutex
}

// NewClaudieCloudProvider returns a ClaudieCloudProvider with initialized caches.
func NewClaudieCloudProvider(ctx context.Context, projectName, clusterName string) *ClaudieCloudProvider {
	// Connect to Claudie and retrieve *pb.K8Scluster
	var (
		cluster    *spec.K8Scluster
		err        error
		rName      string
		rNamespace string
		nm         *node_manager.NodeManager
	)
	if cluster, rName, rNamespace, err = getClaudieState(ctx, projectName, clusterName); err != nil {
		panic(fmt.Sprintf("Error while getting cluster %s : %v", clusterName, err))
	}
	if nm, err = node_manager.NewNodeManager(cluster.ClusterInfo.NodePools); err != nil {
		panic(fmt.Sprintf("Error while creating node manager : %v", err))
	}
	// Initialize all other variables.
	log.Logger = log.Logger.With().Str("cluster", cluster.ClusterInfo.Id()).Logger()
	return &ClaudieCloudProvider{
		projectName:       projectName,
		configCluster:     cluster,
		resourceName:      rName,
		resourceNamespace: rNamespace,
		nodesCache:        getNodesCache(cluster.ClusterInfo.NodePools),
		nodeManager:       nm,
	}
}

// getClaudieState returns a *pb.K8Scluster, resourceName and resourceNamespace from Claudie, for this particular ClaudieCloudProvider instance.
func getClaudieState(ctx context.Context, projectName, clusterName string) (*spec.K8Scluster, string, string, error) {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to conntect to manager:%w", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			log.Err(err).Msgf("Failed to close manager connection")
		}
	}()

	resp, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: projectName})
	if err != nil {
		if errors.Is(err, managerclient.ErrNotFound) {
			log.Warn().Msgf("%v", err)
		}
		return nil, "", "", fmt.Errorf("failed to get config %s : %w", projectName, err)
	}

	if resp.Config.Manifest.Raw == "" && len(resp.Config.Manifest.Checksum) == 0 {
		return nil, "", "", ErrConfigInDeletion
	}

	for cluster, state := range resp.Config.Clusters {
		if k8s := state.GetCurrent().GetK8S(); cluster == clusterName && k8s != nil {
			return k8s, resp.Config.K8SCtx.Name, resp.Config.K8SCtx.Namespace, nil
		}
	}

	return nil, "", "", fmt.Errorf("failed to find cluster %s in config for a project %s", clusterName, projectName)
}

// getNodesCache returns a map of nodeCache, regarding all information needed based on the nodepools with autoscaling enabled.
func getNodesCache(nodepools []*spec.NodePool) map[string]*nodeCache {
	var nc = make(map[string]*nodeCache, len(nodepools))
	for _, np := range nodepools {
		if np.GetDynamicNodePool() != nil {
			// Cache nodepools, which are autoscaled.
			if np.GetDynamicNodePool().AutoscalerConfig != nil {
				// Create nodeGroup struct.
				ng := &protos.NodeGroup{
					Id:      np.Name,
					MinSize: np.GetDynamicNodePool().AutoscalerConfig.Min,
					MaxSize: np.GetDynamicNodePool().AutoscalerConfig.Max,
					Debug:   fmt.Sprintf("Nodepool %s [min %d, max %d]", np.Name, np.GetDynamicNodePool().AutoscalerConfig.Min, np.GetDynamicNodePool().AutoscalerConfig.Max),
				}
				// Append ng to the final slice.
				nc[np.Name] = &nodeCache{nodeGroup: ng, nodepool: np, targetSize: np.GetDynamicNodePool().Count}
			}
		}
	}
	return nc
}

// NodeGroups returns all node groups configured for this cloud provider.
func (c *ClaudieCloudProvider) NodeGroups(_ context.Context, req *protos.NodeGroupsRequest) (*protos.NodeGroupsResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroups request")
	ngs := make([]*protos.NodeGroup, 0, len(c.nodesCache))
	for _, ngc := range c.nodesCache {
		ngs = append(ngs, ngc.nodeGroup)
	}
	return &protos.NodeGroupsResponse{NodeGroups: ngs}, nil
}

// NodeGroupForNode returns the node group for the given node.
// The node group id is an empty string if the node should not
// be processed by cluster autoscaler.
func (c *ClaudieCloudProvider) NodeGroupForNode(_ context.Context, req *protos.NodeGroupForNodeRequest) (*protos.NodeGroupForNodeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got NodeGroupForNode request")
	nodeName := req.Node.Name
	// Initialize as empty response.
	nodeGroup := &protos.NodeGroup{}
	// Try to find if node is from any NodeGroup
	for id, ngc := range c.nodesCache {
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
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got PricingNodePrice request; Not implemented")
	return nil, ErrNotImplemented
}

// PricingPodPrice returns a theoretical minimum price of running a pod for a given
// period of time on a perfectly matching machine.
// Implementation optional.
func (c *ClaudieCloudProvider) PricingPodPrice(_ context.Context, req *protos.PricingPodPriceRequest) (*protos.PricingPodPriceResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got PricingPodPrice request; Not implemented")
	return nil, ErrNotImplemented
}

// GPULabel returns the label added to nodes with GPU resource.
func (c *ClaudieCloudProvider) GPULabel(_ context.Context, req *protos.GPULabelRequest) (*protos.GPULabelResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got GPULabel request")
	return &protos.GPULabelResponse{Label: GpuLabel}, nil
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (c *ClaudieCloudProvider) GetAvailableGPUTypes(_ context.Context, req *protos.GetAvailableGPUTypesRequest) (*protos.GetAvailableGPUTypesResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got GetAvailableGPUTypes request")
	return &protos.GetAvailableGPUTypesResponse{}, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (c *ClaudieCloudProvider) Cleanup(_ context.Context, req *protos.CleanupRequest) (*protos.CleanupResponse, error) {
	log.Info().Msgf("Got Cleanup request")
	return &protos.CleanupResponse{}, nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *ClaudieCloudProvider) Refresh(ctx context.Context, _ *protos.RefreshRequest) (*protos.RefreshResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Info().Msgf("Got Refresh request")
	return &protos.RefreshResponse{}, c.refresh(ctx)
}

// refresh refreshes the state of the claudie provider based of the state from Claudie.
func (c *ClaudieCloudProvider) refresh(ctx context.Context) error {
	log.Info().Msgf("Refreshing the state")

	cluster, rName, rNamespace, err := getClaudieState(ctx, c.projectName, c.configCluster.ClusterInfo.Name)
	if errors.Is(err, ErrConfigInDeletion) {
		log.Debug().Msgf("config for cluster %s is being deleted skipping refresh", c.configCluster.ClusterInfo.Name)
		return nil
	}
	if err != nil {
		log.Err(err).Msgf("Error while refreshing a state of the cluster")
		return fmt.Errorf("error while refreshing a state for the cluster %s : %w", c.configCluster.ClusterInfo.Name, err)
	}

	c.configCluster = cluster
	c.resourceName = rName
	c.resourceNamespace = rNamespace
	c.nodesCache = getNodesCache(cluster.ClusterInfo.NodePools)

	if err := c.nodeManager.Refresh(cluster.ClusterInfo.NodePools); err != nil {
		return fmt.Errorf("failed to refresh node manager : %w", err)
	}

	return nil
}

// SendAutoscalerEvent will sent the resourceName and resourceNamespace to the InputManifest controller,
// when a scaleup or scaledown occurs
func (c *ClaudieCloudProvider) sendAutoscalerEvent() error {
	var cc *grpc.ClientConn
	var err error
	operatorURL := strings.ReplaceAll(envs.OperatorURL, ":tcp://", "")
	log.Info().Msgf("Sending autoscale event to %s: %s, %s, ", operatorURL, c.resourceName, c.resourceNamespace)
	if cc, err = grpcutils.GrpcDialWithRetryAndBackoff("claudie-operator", operatorURL); err != nil {
		return fmt.Errorf("failed to dial claudie-operator at %s : %w", envs.OperatorURL, err)
	}
	defer cc.Close()

	client := pb.NewOperatorServiceClient(cc)
	if err := cOperator.SendAutoscalerEvent(client, &pb.SendAutoscalerEventRequest{
		InputManifestName:      c.resourceName,
		InputManifestNamespace: c.resourceNamespace,
	}); err != nil {
		return fmt.Errorf("error while sending autoscaling event to claudie-operator : %w", err)
	}
	return nil
}
