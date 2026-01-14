package claudie_provider

// import (
// 	"context"
// 	"errors"
// 	"fmt"
// 	"slices"
// 	"strings"
// 	"sync"

// 	"github.com/berops/claudie/internal/envs"
// 	"github.com/berops/claudie/internal/grpcutils"
// 	"github.com/berops/claudie/internal/nodepools"
// 	"github.com/berops/claudie/proto/pb"
// 	"github.com/berops/claudie/proto/pb/spec"
// 	"github.com/berops/claudie/services/autoscaler-adapter/node_manager"
// 	cOperator "github.com/berops/claudie/services/claudie-operator/client"
// 	managerclient "github.com/berops/claudie/services/managerv2/client"
// 	"github.com/rs/zerolog/log"

// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/codes"
// 	"google.golang.org/grpc/status"

// 	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
// )

// // TODO: I believe this is wrong and we might need to store this into persistent storage
// // i.e what happens when the services crashes will it remember the deletion ? claude
// // says no.
// //
// // Maybe we could really make calls to the manager and storing the information in the
// // database along the current state...

// // TODO: I thing we need to add a targetSize to the autoscaler config
// // and check that as the desired state of nodes to be created.
// // And we could then match it with the count which represent the actuall number of nodes
// // not the desired target size.
// //
// // And in the autoscaler we can then see count < targetSize the remaining nodes could be creating
// //
// //	count > targetSize the nodes will be in deleting ?
// //
// // now to decide which will be which is a hard one also what will be the names of the "creating nodes"
// // and what will be the name of the "deleting nodes".
// //
// // But then also how would the DeleteNodes work which needs to propagate the actuall Node IDs for deletion
// // I think we really need
// // TODO: read this i guess.
// const (
// 	// Default GPU label.
// 	GpuLabel = "claudie.io/gpu-node"
// )

// var (
// 	// ErrConfigInDeletion returned when the desired state of the config is nil and cannot perform a refresh operation.
// 	ErrConfigInDeletion = errors.New("config is marked for deletion")
// )

// type ImmutableState struct {
// 	// State as per Cluster Autoscaler definition.
// 	NodeGroup *protos.NodeGroup

// 	// State as per Claudie definition.
// 	Nodepool *spec.NodePool
// }

// type MutableState struct {
// 	// Target size of node group.
// 	//
// 	// How many nodes should the nodepool
// 	// currently have. Changes based on
// 	// the autoscaler requirements.
// 	TargetSize int32

// 	// Nodes to delete, populated based on
// 	// the autoscaler requiremments.
// 	NodesToDelete map[string]struct{}
// }

// // State represents the state of the nodepool from different sources.
// type State struct {
// 	// State is is not modified once assigned.
// 	Immutable ImmutableState

// 	// State is can be mutated based on the
// 	// autoscaler events or sync with the claudie
// 	// version of the nodepool.
// 	Mutable MutableState
// }

// type ClaudieCloudProvider struct {
// 	protos.UnimplementedCloudProviderServer

// 	// Name of the Claudie config.
// 	projectName string

// 	// Kubernetes InputManifest resource name
// 	resourceName string

// 	// Kubernetes InputManifest resource namespace
// 	resourceNamespace string

// 	// Cluster as described in Claudie config.
// 	clusterName string
// 	clusterHash string

// 	// Map of cached info regarding nodes.
// 	managedPools map[string]*State

// 	// Node manager.
// 	nodeManager *node_manager.NodeManager

// 	// Server mutex
// 	lock sync.Mutex
// }

// // NewClaudieCloudProvider returns a ClaudieCloudProvider with initialized caches.
// func NewClaudieCloudProvider(ctx context.Context, projectName, clusterName string) (*ClaudieCloudProvider, error) {
// 	cluster, rName, rNamespace, err := claudieCurrentState(ctx, projectName, clusterName)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed retrieving cluster %s: %w", clusterName, err)
// 	}

// 	p := &ClaudieCloudProvider{
// 		projectName:       projectName,
// 		clusterName:       cluster.ClusterInfo.Name,
// 		clusterHash:       cluster.ClusterInfo.Hash,
// 		resourceName:      rName,
// 		resourceNamespace: rNamespace,
// 		managedPools:      make(map[string]*State),
// 		nodeManager:       node_manager.NewNodeManager(),
// 	}

// 	autoscaled := nodepools.Autoscaled(cluster.ClusterInfo.NodePools)
// 	if err := p.nodeManager.Refresh(autoscaled); err != nil {
// 		return nil, fmt.Errorf("failed to fetch instance info for autoscaled nodepools: %w", err)
// 	}

// 	// Build the initial state for the current autoscaled nodepools.
// 	// New nodepools will then be periodically checked for in the
// 	// [ClaudieCloudProvider.Refresh] function. The [Mutable] state
// 	// will be managed based on the received autoscaler events. The
// 	// [Immutable] state never changes once Assigned, though it may be
// 	// re-assigned if the range of the nodes changes for the nodepool
// 	// during the [ClaudieCloudProvider.Refresh] function.
// 	for _, np := range autoscaled {
// 		dyn := np.GetDynamicNodePool()

// 		ng := &protos.NodeGroup{
// 			Id:      np.Name,
// 			MinSize: dyn.AutoscalerConfig.Min,
// 			MaxSize: dyn.AutoscalerConfig.Max,
// 			Debug: fmt.Sprintf("Nodepool %s [min %d, max %d]",
// 				np.Name,
// 				dyn.AutoscalerConfig.Min,
// 				dyn.AutoscalerConfig.Max,
// 			),
// 		}

// 		p.managedPools[np.Name] = &State{
// 			Immutable: ImmutableState{
// 				NodeGroup: ng,
// 				Nodepool:  np,
// 			},
// 			Mutable: MutableState{
// 				TargetSize:    dyn.Count,
// 				NodesToDelete: nil,
// 			},
// 		}
// 	}

// 	log.Logger = log.Logger.With().Str("cluster", cluster.ClusterInfo.Id()).Logger()

// 	return p, nil
// }

// // Returns the current state of the a *pb.K8Scluster, resourceName and resourceNamespace from Claudie, for this particular ClaudieCloudProvider instance.
// func claudieCurrentState(ctx context.Context, projectName, clusterName string) (*spec.K8Scluster, string, string, error) {
// 	manager, err := managerclient.New(&log.Logger)
// 	if err != nil {
// 		return nil, "", "", fmt.Errorf("failed to conntect to manager:%w", err)
// 	}
// 	defer func() {
// 		if err := manager.Close(); err != nil {
// 			log.Err(err).Msgf("Failed to close manager connection")
// 		}
// 	}()

// 	req := managerclient.GetConfigRequest{Name: projectName}
// 	resp, err := manager.GetConfig(ctx, &req)
// 	if err != nil {
// 		if errors.Is(err, managerclient.ErrNotFound) {
// 			log.Warn().Msgf("%v", err)
// 		}
// 		return nil, "", "", fmt.Errorf("failed to get config %s : %w", projectName, err)
// 	}

// 	if resp.Config.Manifest.Raw == "" && len(resp.Config.Manifest.Checksum) == 0 {
// 		return nil, "", "", ErrConfigInDeletion
// 	}

// 	for cluster, state := range resp.Config.Clusters {
// 		if k8s := state.GetCurrent().GetK8S(); cluster == clusterName && k8s != nil {
// 			return k8s, resp.Config.K8SCtx.Name, resp.Config.K8SCtx.Namespace, nil
// 		}
// 	}

// 	return nil, "", "", fmt.Errorf("failed to find cluster %s in config for a project %s", clusterName, projectName)
// }

// // // From the autoscaled nodepools builds a cache that maps together the Claudie version of the
// // // nodepool and the cluster-autoscaler version of the nodepool along with the target size.
// // func buildState(autoscaled []*spec.NodePool) map[string]*State {
// // 	var nc = make(map[string]*State, len(autoscaled))

// // 	for _, np := range autoscaled {
// // 		dyn := np.GetDynamicNodePool()

// // 		ng := &protos.NodeGroup{
// // 			Id:      np.Name,
// // 			MinSize: dyn.AutoscalerConfig.Min,
// // 			MaxSize: dyn.AutoscalerConfig.Max,
// // 			Debug:   fmt.Sprintf("Nodepool %s [min %d, max %d]", np.Name, dyn.AutoscalerConfig.Min, dyn.AutoscalerConfig.Max),
// // 		}

// // 		nc[np.Name] = &State{
// // 			Immutable: ImmutableState{
// // 				NodeGroup: ng,
// // 				Nodepool:  np,
// // 			},
// // 			Mutable: MutableState{
// // 				TargetSize:    dyn.Count,
// // 				NodesToDelete: nil,
// // 			},
// // 		}
// // 	}

// // 	return nc
// // }

// // NodeGroups returns all node groups configured for this cloud provider.
// func (c *ClaudieCloudProvider) NodeGroups(_ context.Context, req *protos.NodeGroupsRequest) (*protos.NodeGroupsResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got NodeGroups request")

// 	ngs := make([]*protos.NodeGroup, 0, len(c.managedPools))

// 	for _, ngc := range c.managedPools {
// 		ngs = append(ngs, &protos.NodeGroup{
// 			Id:      ngc.Immutable.NodeGroup.Id,
// 			MinSize: ngc.Immutable.NodeGroup.MinSize,
// 			MaxSize: ngc.Immutable.NodeGroup.MaxSize,
// 			Debug:   ngc.Immutable.NodeGroup.Debug,
// 		})
// 	}

// 	return &protos.NodeGroupsResponse{NodeGroups: ngs}, nil
// }

// // NodeGroupForNode returns the node group for the given node.
// // The node group id is an empty string if the node should not
// // be processed by cluster autoscaler.
// func (c *ClaudieCloudProvider) NodeGroupForNode(_ context.Context, req *protos.NodeGroupForNodeRequest) (*protos.NodeGroupForNodeResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got NodeGroupForNode request")

// 	nodeName := req.Node.Name

// 	// Initialize as empty response.
// 	nodeGroup := &protos.NodeGroup{}

// 	// Try to find if node is from any NodeGroup
// 	for id, ngc := range c.managedPools {
// 		// If node name contains ng.Id (nodepool name), return this NodeGroup.
// 		if strings.Contains(nodeName, id) {
// 			nodeGroup = &protos.NodeGroup{
// 				Id:      ngc.Immutable.NodeGroup.Id,
// 				MinSize: ngc.Immutable.NodeGroup.MinSize,
// 				MaxSize: ngc.Immutable.NodeGroup.MaxSize,
// 				Debug:   ngc.Immutable.NodeGroup.Debug,
// 			}
// 			break
// 		}
// 	}

// 	return &protos.NodeGroupForNodeResponse{NodeGroup: nodeGroup}, nil
// }

// // PricingNodePrice returns a theoretical minimum price of running a node for
// // a given period of time on a perfectly matching machine.
// // Implementation optional.
// func (c *ClaudieCloudProvider) PricingNodePrice(_ context.Context, req *protos.PricingNodePriceRequest) (*protos.PricingNodePriceResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()
// 	log.Info().Msgf("Got PricingNodePrice request; Not implemented")
// 	return nil, status.Error(codes.Unimplemented, "Pricing unimplemented")
// }

// // PricingPodPrice returns a theoretical minimum price of running a pod for a given
// // period of time on a perfectly matching machine.
// // Implementation optional.
// func (c *ClaudieCloudProvider) PricingPodPrice(_ context.Context, req *protos.PricingPodPriceRequest) (*protos.PricingPodPriceResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()
// 	log.Info().Msgf("Got PricingPodPrice request; Not implemented")
// 	return nil, status.Error(codes.Unimplemented, "Pricing unimplemented")
// }

// // GPULabel returns the label added to nodes with GPU resource.
// func (c *ClaudieCloudProvider) GPULabel(_ context.Context, req *protos.GPULabelRequest) (*protos.GPULabelResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()
// 	log.Info().Msgf("Got GPULabel request")
// 	return &protos.GPULabelResponse{Label: GpuLabel}, nil
// }

// // GetAvailableGPUTypes return all available GPU types cloud provider supports.
// func (c *ClaudieCloudProvider) GetAvailableGPUTypes(_ context.Context, req *protos.GetAvailableGPUTypesRequest) (*protos.GetAvailableGPUTypesResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()
// 	log.Info().Msgf("Got GetAvailableGPUTypes request")
// 	return &protos.GetAvailableGPUTypesResponse{}, nil
// }

// // Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
// func (c *ClaudieCloudProvider) Cleanup(_ context.Context, req *protos.CleanupRequest) (*protos.CleanupResponse, error) {
// 	log.Info().Msgf("Got Cleanup request")
// 	return &protos.CleanupResponse{}, nil
// }

// // Refresh is called before every main loop and can be used to dynamically
// // update cloud provider state. In particular the list of node groups returned
// // by NodeGroups() can change as a result of CloudProvider.Refresh().
// func (c *ClaudieCloudProvider) Refresh(ctx context.Context, _ *protos.RefreshRequest) (*protos.RefreshResponse, error) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()

// 	log.Info().Msgf("Got Refresh request")

// 	// 1. read the current state of the cluster as claudie knows it.
// 	cluster, rName, rNamespace, err := claudieCurrentState(ctx, c.projectName, c.clusterName)
// 	if errors.Is(err, ErrConfigInDeletion) {
// 		log.Debug().Msgf("config for cluster %s is being deleted skipping refresh", c.clusterName)
// 		return &protos.RefreshResponse{}, nil
// 	}
// 	if err != nil {
// 		log.Err(err).Msgf("Error while refreshing a state of the cluster")
// 		return nil, fmt.Errorf("error while refreshing a state for the cluster %s : %w", c.clusterName, err)
// 	}

// 	// 2. refresh node manager cache, possibly catching new nodepools.
// 	autoscaled := nodepools.Autoscaled(cluster.ClusterInfo.NodePools)
// 	if err := c.nodeManager.Refresh(autoscaled); err != nil {
// 		return nil, fmt.Errorf("failed to fetch instance info for autoscaled nodepools: %w", err)
// 	}

// 	// 3. For each of the autoscaled check if deletions were made, if any.
// 	for _, refreshed := range autoscaled {
// 		dyn := refreshed.GetDynamicNodePool()

// 		if lastKnown, ok := c.managedPools[refreshed.Name]; ok {
// 			// Check if any pending deletions were done by
// 			// claudie if yes remove them.
// 			var removedNodes []string

// 			for node := range lastKnown.Mutable.NodesToDelete {
// 				exists := slices.ContainsFunc(refreshed.Nodes, func(n *spec.Node) bool {
// 					return n.Name == node
// 				})
// 				if !exists {
// 					removedNodes = append(removedNodes, node)
// 				}
// 			}

// 			for _, n := range removedNodes {
// 				delete(lastKnown.Mutable.NodesToDelete, n)
// 			}
// 		}
// 	}

// 	return &protos.RefreshResponse{}, nil
// }

// // refresh refreshes the state of the claudie provider based of the state from Claudie.
// func (c *ClaudieCloudProvider) refresh(ctx context.Context) error {
// 	log.Info().Msgf("Refreshing the state")

// 	managedPools := buildState(autoscaled)

// 	c.clusterName = cluster.ClusterInfo.Name
// 	c.clusterHash = cluster.ClusterInfo.Hash
// 	c.resourceName = rName
// 	c.resourceNamespace = rNamespace
// 	c.managedPools = managedPools

// 	return nil
// }

// // SendAutoscalerEvent will sent the resourceName and resourceNamespace to the InputManifest controller,
// // when a scaleup or scaledown occurs
// func (c *ClaudieCloudProvider) sendAutoscalerEvent() error {
// 	var cc *grpc.ClientConn
// 	var err error

// 	operatorURL := strings.ReplaceAll(envs.OperatorURL, ":tcp://", "")

// 	log.Info().Msgf("Sending autoscale event to %s: %s, %s, ", operatorURL, c.resourceName, c.resourceNamespace)

// 	if cc, err = grpcutils.GrpcDialWithRetryAndBackoff("claudie-operator", operatorURL); err != nil {
// 		return fmt.Errorf("failed to dial claudie-operator at %s : %w", envs.OperatorURL, err)
// 	}
// 	defer cc.Close()

// 	client := pb.NewOperatorServiceClient(cc)
// 	req := pb.SendAutoscalerEventRequest{
// 		InputManifestName:      c.resourceName,
// 		InputManifestNamespace: c.resourceNamespace,
// 	}

// 	if err := cOperator.SendAutoscalerEvent(client, &req); err != nil {
// 		return fmt.Errorf("error while sending autoscaling event to claudie-operator : %w", err)
// 	}

// 	return nil
// }
