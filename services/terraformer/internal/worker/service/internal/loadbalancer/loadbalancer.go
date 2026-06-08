package loadbalancer

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/nodepools"
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
	Cluster     *spec.LBcluster

	// NodePools that were deleted and are no longer part of the
	// passed in [LBcluster.Cluster] state, but still need their
	// infra to be removed.
	//
	// This field needs to be set for nodepools that were deleted
	// as the provider will be missing from the [K8Scluster.Cluster]
	// state and it will fail on the clean up.
	GhostNodePools []*spec.NodePool

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
		ClusterName:    l.Cluster.ClusterInfo.Name,
		ClusterHash:    l.Cluster.ClusterInfo.Hash,
		ClusterId:      l.Cluster.ClusterInfo.Id(),
		NodePools:      l.Cluster.ClusterInfo.NodePools,
		GhostNodePools: l.GhostNodePools,
		ProjectName:    projectName,
		ClusterType:    cluster_builder.LoadBalancer,
		LBInfo: cluster_builder.LBInfo{
			Roles: roles,
		},
		SpawnProcessLimit: processLimit,
	}

	if err := clusterBuilder.ReconcileNodePools(); err != nil {
		return fmt.Errorf("%w: error while creating the LB cluster %s : %w", ErrCreateNodePools, ci.Name, err)
	}

	nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools)
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
		nodeIPs      = nodepools.PublicEndpoints(ci.NodePools)
	)

	group := errgroup.Group{}
	group.Go(func() error {
		cluster := cluster_builder.ClusterBuilder{
			ClusterName:       l.Cluster.ClusterInfo.Name,
			ClusterHash:       l.Cluster.ClusterInfo.Hash,
			ClusterId:         l.Cluster.ClusterInfo.Id(),
			NodePools:         l.Cluster.ClusterInfo.NodePools,
			ProjectName:       projectName,
			ClusterType:       cluster_builder.LoadBalancer,
			SpawnProcessLimit: processLimit,

			// during deletion these are not required to be set as deletion works
			// always with the current state.
			GhostNodePools: nil,
		}
		return cluster.DestroyNodepools()
	})

	group.Go(func() error {
		if l.Cluster.Dns == nil {
			return nil
		}

		var emptycount int
		for _, ip := range nodeIPs {
			if ip == "" {
				emptycount += 1
			}
		}

		// This check needs to be done as the resources in the terraform
		// templates are based on IPs and if there are more than two nodes
		// that do not have an IP it will continuously fail. But given that
		// the DNS isn't even build if atleast 1 node fails means that this
		// will only be hit if the building of the infrastructure failed altogether
		// and to avoid error simply do not destroy what was not build.
		//
		// This really depends on how the templates are structured but this catches
		// the case where the IP is used in the resource name.
		if emptycount > 1 {
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
