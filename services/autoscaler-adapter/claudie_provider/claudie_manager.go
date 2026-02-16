package claudie_provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/autoscaler-adapter/node_manager"
	cOperator "github.com/berops/claudie/services/claudie-operator/client"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

var (
	// ErrConfigInDeletion returned when the desired state of the config is nil and cannot perform a refresh operation.
	ErrConfigInDeletion = errors.New("config is marked for deletion")
)

type K8sCtx struct {
	ResourceName      string
	ResourceNamespace string
}

type Immutable struct {
	Config      string
	ClusterName string
	ClusterHash string
}

type Manager struct {
	protos.UnimplementedCloudProviderServer

	// Once assigned never changes.
	// Does not need to be covered by a Mutex.
	Immutable Immutable

	K8sCtx      K8sCtx
	NodeManager *node_manager.NodeManager
	Groups      map[string]*NodeGroup

	lock sync.Mutex
}

func NewProvider(ctx context.Context, config string, clusterName string) (*Manager, error) {
	cluster, kctx, err := claudieCurrentState(ctx, config, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve current state for %q: %w", clusterName, err)
	}

	m := &Manager{
		K8sCtx: kctx,
		Immutable: Immutable{
			Config:      config,
			ClusterName: cluster.ClusterInfo.Name,
			ClusterHash: cluster.ClusterInfo.Hash,
		},
		NodeManager: node_manager.NewNodeManager(),
		Groups:      make(map[string]*NodeGroup),
	}

	autoscaled := nodepools.Autoscaled(cluster.ClusterInfo.NodePools)
	if err := m.NodeManager.Refresh(autoscaled); err != nil {
		return nil, fmt.Errorf("failed to fetch instance info for autoscaled nodepools: %w", err)
	}

	// Build the initial state for the current autoscaled nodepools.
	// New nodepools will then be periodically checked for in the
	// [Manager.Refresh] function.
	for _, np := range autoscaled {
		dyn := np.GetDynamicNodePool()

		g := protos.NodeGroup{
			Id:      np.Name,
			MinSize: dyn.AutoscalerConfig.Min,
			MaxSize: dyn.AutoscalerConfig.Max,
			Debug: fmt.Sprintf("Nodepool %s [min %d, max %d]",
				np.Name,
				dyn.AutoscalerConfig.Min,
				dyn.AutoscalerConfig.Max,
			),
		}

		m.Groups[np.Name] = &NodeGroup{
			G:          &g,
			N:          np,
			TargetSize: dyn.AutoscalerConfig.TargetSize,
		}
	}

	log.Logger = log.
		Logger.
		With().
		Str("cluster", cluster.ClusterInfo.Id()).
		Logger()

	return m, nil
}

// Returns the current state of the a *pb.K8Scluster, resourceName and resourceNamespace
// from Claudie, for this particular ClaudieCloudProvider instance. If the config is in
// deletion the [ErrConfigInDeletion] is returned.
func claudieCurrentState(ctx context.Context, config, clusterName string) (*spec.K8Scluster, K8sCtx, error) {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return nil, K8sCtx{}, fmt.Errorf("failed to conntect to manager:%w", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			log.Err(err).Msgf("Failed to close manager connection")
		}
	}()

	req := managerclient.GetConfigRequest{Name: config}
	resp, err := manager.GetConfig(ctx, &req)
	if err != nil {
		if errors.Is(err, managerclient.ErrNotFound) {
			log.Warn().Msgf("%v", err)
		}
		return nil, K8sCtx{}, fmt.Errorf("failed to get config %s : %w", config, err)
	}

	if resp.Config.Manifest.Raw == "" && len(resp.Config.Manifest.Checksum) == 0 {
		return nil, K8sCtx{}, ErrConfigInDeletion
	}

	for cluster, state := range resp.Config.Clusters {
		if k8s := state.GetCurrent().GetK8S(); cluster == clusterName && k8s != nil {
			kctx := K8sCtx{
				ResourceName:      resp.Config.K8SCtx.Name,
				ResourceNamespace: resp.Config.K8SCtx.Namespace,
			}
			return k8s, kctx, nil
		}
	}

	return nil, K8sCtx{}, fmt.Errorf("failed to find cluster %s in config for a project %s", clusterName, config)
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (m *Manager) Refresh(ctx context.Context, _ *protos.RefreshRequest) (*protos.RefreshResponse, error) {
	log.Info().Msg("Handling Refresh request")

	cfg, cname := m.Immutable.Config, m.Immutable.ClusterName
	cluster, kctx, err := claudieCurrentState(ctx, cfg, cname)
	if err != nil {
		if errors.Is(err, ErrConfigInDeletion) {
			log.
				Info().
				Msgf("Config for cluster %q is in deletion, skipping refresh", cname)
			return &protos.RefreshResponse{}, nil
		}

		log.Err(err).Msg("Error while refreshing state of cluster")
		return nil, fmt.Errorf("failed to refresh cluster state for %q: %w", cname, err)
	}

	autoscaled := nodepools.Autoscaled(cluster.ClusterInfo.NodePools)

	group := make(map[string]*NodeGroup)
	for _, refreshed := range autoscaled {
		dyn := refreshed.GetDynamicNodePool()

		g := protos.NodeGroup{
			Id:      refreshed.Name,
			MinSize: dyn.AutoscalerConfig.Min,
			MaxSize: dyn.AutoscalerConfig.Max,
			Debug: fmt.Sprintf("Nodepool %s [min %d, max %d]",
				refreshed.Name,
				dyn.AutoscalerConfig.Min,
				dyn.AutoscalerConfig.Max,
			),
		}

		group[refreshed.Name] = &NodeGroup{
			G:          &g,
			N:          refreshed,
			TargetSize: dyn.AutoscalerConfig.TargetSize,
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	if err := m.NodeManager.Refresh(autoscaled); err != nil {
		return nil, fmt.Errorf("failed to refresh instance info cache for %q: %w", cname, err)
	}

	m.K8sCtx = kctx
	m.Groups = group

	return &protos.RefreshResponse{}, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (c *Manager) Cleanup(_ context.Context, _ *protos.CleanupRequest) (*protos.CleanupResponse, error) {
	log.Info().Msgf("Handling Cleanup request")
	return &protos.CleanupResponse{}, nil
}

// SendAutoscalerEvent will sent the resourceName and resourceNamespace to the InputManifest controller,
// when a scaleup or scaledown occurs
func sendAutoscalerEvent(kctx K8sCtx) error {
	var cc *grpc.ClientConn
	var err error

	operatorURL := strings.ReplaceAll(envs.OperatorURL, ":tcp://", "")

	log.
		Info().
		Msgf("Sending autoscale event to %s: %s, %s, ", operatorURL, kctx.ResourceName, kctx.ResourceNamespace)

	if cc, err = grpcutils.GrpcDialWithRetryAndBackoff("claudie-operator", operatorURL); err != nil {
		return fmt.Errorf("failed to dial claudie-operator at %s : %w", envs.OperatorURL, err)
	}
	defer cc.Close()

	client := pb.NewOperatorServiceClient(cc)
	req := pb.SendAutoscalerEventRequest{
		InputManifestName:      kctx.ResourceName,
		InputManifestNamespace: kctx.ResourceNamespace,
	}

	if err := cOperator.SendAutoscalerEvent(client, &req); err != nil {
		return fmt.Errorf("error while sending autoscaling event to claudie-operator : %w", err)
	}

	return nil
}
