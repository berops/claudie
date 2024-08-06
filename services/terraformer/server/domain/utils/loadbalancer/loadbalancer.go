package loadbalancer

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/server/domain/utils/cluster-builder"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

var (
	// ErrCreateDNSRecord is returned when an error occurs during the creation of the DNS records
	ErrCreateDNSRecord = errors.New("failed to create DNS record")
)

type LBcluster struct {
	ProjectName string

	DesiredState *spec.LBcluster
	CurrentState *spec.LBcluster

	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values should always be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

func (l *LBcluster) Id() string {
	state := l.DesiredState
	if state == nil {
		state = l.CurrentState
	}

	return utils.GetClusterID(state.ClusterInfo)
}

func (l *LBcluster) Build(logger zerolog.Logger) error {
	logger.Info().Msgf("Building LB Cluster %s and DNS", l.DesiredState.ClusterInfo.Name)

	var currentClusterInfo *spec.ClusterInfo
	var currentDNS *spec.DNS
	var currentNodeIPs []string

	// Check if current cluster was defined, to avoid access of unrefferenced memory
	if l.CurrentState != nil {
		currentClusterInfo = l.CurrentState.ClusterInfo
		currentDNS = l.CurrentState.Dns
		currentNodeIPs = getNodeIPs(l.CurrentState.ClusterInfo.NodePools)
	}

	clusterBuilder := cluster_builder.ClusterBuilder{
		DesiredClusterInfo: l.DesiredState.ClusterInfo,
		CurrentClusterInfo: currentClusterInfo,
		ProjectName:        l.ProjectName,
		ClusterType:        spec.ClusterType_LB,
		LBInfo: cluster_builder.LBInfo{
			Roles: l.DesiredState.Roles,
		},
		SpawnProcessLimit: l.SpawnProcessLimit,
	}

	if err := clusterBuilder.CreateNodepools(); err != nil {
		return fmt.Errorf("error while creating the LB cluster %s : %w", l.DesiredState.ClusterInfo.Name, err)
	}

	nodeIPs := getNodeIPs(l.DesiredState.ClusterInfo.NodePools)
	dns := DNS{
		ClusterName:       l.DesiredState.ClusterInfo.Name,
		ClusterHash:       l.DesiredState.ClusterInfo.Hash,
		CurrentNodeIPs:    currentNodeIPs,
		DesiredNodeIPs:    nodeIPs,
		CurrentDNS:        currentDNS,
		DesiredDNS:        l.DesiredState.Dns,
		ProjectName:       l.ProjectName,
		SpawnProcessLimit: l.SpawnProcessLimit,
	}

	endpoint, err := dns.CreateDNSRecords(logger)
	if err != nil {
		return fmt.Errorf("%w for %s: %w", ErrCreateDNSRecord, l.DesiredState.ClusterInfo.Name, err)
	}

	l.DesiredState.Dns.Endpoint = endpoint

	return nil
}

func (l *LBcluster) Destroy(logger zerolog.Logger) error {
	group := errgroup.Group{}
	logger.Info().Msgf("Destroying LB Cluster %s and DNS", l.CurrentState.ClusterInfo.Name)

	group.Go(func() error {
		cluster := cluster_builder.ClusterBuilder{
			CurrentClusterInfo: l.CurrentState.ClusterInfo,
			ProjectName:        l.ProjectName,
			ClusterType:        spec.ClusterType_LB,
			SpawnProcessLimit:  l.SpawnProcessLimit,
		}
		return cluster.DestroyNodepools()
	})

	group.Go(func() error {
		dns := DNS{
			ClusterName:       l.CurrentState.ClusterInfo.Name,
			ClusterHash:       l.CurrentState.ClusterInfo.Hash,
			CurrentNodeIPs:    getNodeIPs(l.CurrentState.ClusterInfo.NodePools),
			CurrentDNS:        l.CurrentState.Dns,
			ProjectName:       l.ProjectName,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}
		return dns.DestroyDNSRecords(logger)
	})

	return group.Wait()
}

func (l *LBcluster) UpdateCurrentState() { l.CurrentState = l.DesiredState }

// getNodeIPs returns slice of public IPs used in the node pool.
func getNodeIPs(nodepools []*spec.NodePool) []string {
	var ips []string

	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			ips = append(ips, node.Public)
		}
	}

	return ips
}
