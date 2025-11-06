package loadbalancer

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/rs/zerolog"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

var (
	// ErrCreateDNSRecord is returned when an error occurs during the creation of the DNS records
	ErrCreateDNSRecord = errors.New("failed to create DNS record")

	// ErrCreateNodePools is returned when an error occurs during the creation of the desired nodepools.
	ErrCreateNodePools = errors.New("failed to create desired nodepools")
)

type LBcluster struct {
	ProjectName string
	Cluster     *spec.LBclusterV2

	// SpawnProcessLimit  limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (l *LBcluster) Id() string         { return l.Cluster.ClusterInfo.Id() }
func (l *LBcluster) IsKubernetes() bool { return false }

func (l *LBcluster) Build(logger zerolog.Logger) error {
	logger.Info().Msgf("Building LB Cluster %s and DNS", l.Cluster.ClusterInfo.Name)

	var (
		projectName  = l.ProjectName
		ci           = l.Cluster.ClusterInfo
		roles        = l.Cluster.Roles
		processLimit = l.SpawnProcessLimit
	)

	clusterBuilder := cluster_builder.ClusterBuilder{
		ClusterInfo: ci,
		ProjectName: projectName,
		ClusterType: spec.ClusterType_LB,
		LBInfo: cluster_builder.LBInfo{
			Roles: roles,
		},
		SpawnProcessLimit: processLimit,
	}

	if err := clusterBuilder.CreateNodepools(); err != nil {
		return fmt.Errorf("%w: error while creating the LB cluster %s : %w", ErrCreateNodePools, ci.Name, err)
	}

	nodeIPs := getNodeIPs(l.Cluster.ClusterInfo.NodePools)
	dns := DNS{
		ProjectName:       projectName,
		ClusterName:       ci.Name,
		ClusterHash:       ci.Hash,
		NodeIPs:           nodeIPs,
		Dns:               l.Cluster.Dns,
		SpawnProcessLimit: processLimit,
	}

	if err := dns.CreateDNSRecords(logger); err != nil {
		return fmt.Errorf("%w for %s: %w", ErrCreateDNSRecord, ci.Name, err)
	}

	return nil
}

func (l *LBcluster) Destroy(logger zerolog.Logger) error {
	logger.Info().Msgf("Destroying LB Cluster %s and DNS", l.Cluster.ClusterInfo.Name)

	var (
		projectName  = l.ProjectName
		ci           = l.Cluster.ClusterInfo
		processLimit = l.SpawnProcessLimit
		nodeIPs      = getNodeIPs(ci.NodePools)
	)

	group := errgroup.Group{}
	group.Go(func() error {
		cluster := cluster_builder.ClusterBuilder{
			ClusterInfo:       ci,
			ProjectName:       projectName,
			ClusterType:       spec.ClusterType_LB,
			SpawnProcessLimit: processLimit,
		}
		return cluster.DestroyNodepools()
	})

	group.Go(func() error {
		if l.Cluster.Dns == nil {
			return nil
		}

		dns := DNS{
			ProjectName:       projectName,
			ClusterName:       ci.Name,
			ClusterHash:       ci.Hash,
			NodeIPs:           nodeIPs,
			Dns:               l.Cluster.Dns,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}
		return dns.DestroyDNSRecords(logger)
	})

	return group.Wait()
}

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
