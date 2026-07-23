package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/templates"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// The default value on how many nodepools can be built concurrently.
const DefaultNodePoolConcurrencyLimit = 10

type LBcluster struct {
	ProjectName string
	Cluster     *spec.LBcluster

	// How many NodePools can be worked on concurrently.
	//
	// When left uninitialized [DefaultNodePoolConcurrencyLimit] is used as the default
	NodePoolConcurrencyLimit int

	// SpawnProcessLimit  limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (l *LBcluster) Id() string         { return l.Cluster.ClusterInfo.Id() }
func (l *LBcluster) IsKubernetes() bool { return false }

func (l *LBcluster) ReconcileAll(ctx context.Context, logger zerolog.Logger) error {
	logger.Info().Msgf("Building LB Cluster %s and DNS", l.Cluster.ClusterInfo.Name)

	if l.NodePoolConcurrencyLimit == 0 {
		l.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	dynamic := nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
	builder := cluster_builder.ClusterBuilder{
		ClusterName:   l.Cluster.ClusterInfo.Name,
		ClusterHash:   l.Cluster.ClusterInfo.Hash,
		ClusterId:     l.Cluster.ClusterInfo.Id(),
		InputManifest: l.ProjectName,
		Type:          cluster_builder.LoadbalancerCluster,
		LBInfo: cluster_builder.LoadbalancerClusterInfo{
			Roles: l.Cluster.Roles,
		},
		SpawnProcessLimit: l.SpawnProcessLimit,
	}
	dnsBuilder := cluster_builder.DnsBuilder{
		ClusterName:       l.Cluster.ClusterInfo.Name,
		ClusterHash:       l.Cluster.ClusterInfo.Hash,
		ClusterId:         l.Cluster.ClusterInfo.Id(),
		InputManifest:     l.ProjectName,
		SpawnProcessLimit: l.SpawnProcessLimit,
	}

	if err := builder.Init(logger, dynamic); err != nil {
		return err
	}
	defer builder.Done()

	out, err := builder.ReconcileCommon()
	if err != nil {
		return fmt.Errorf("error while reconciling common nodepool infrastructure for %q: %w", builder.ClusterId, err)
	}

	group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
	group.SetLimit(l.NodePoolConcurrencyLimit)

	for i := range dynamic {
		group.Go(func() error { return builder.ReconcileNodePool(out, i) })
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("error while reconciling nodepools for cluster: %q: %w", builder.ClusterId, err)
	}

	nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools) // include all of the nodes, not just dynamic.
	if err := dnsBuilder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
		return err
	}
	defer dnsBuilder.Done()

	if err := dnsBuilder.ReconcileRecords(); err != nil {
		return fmt.Errorf("error while reconciling dns records for cluster %q: %w", builder.ClusterId, err)
	}
	return nil
}

func (l *LBcluster) DestroyAll(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error {
	logger.Info().Msgf("Destroying LB Cluster %s and DNS", l.Cluster.ClusterInfo.Name)

	if l.NodePoolConcurrencyLimit == 0 {
		l.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools)
	dynamic := nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		builder := cluster_builder.ClusterBuilder{
			ClusterName:   l.Cluster.ClusterInfo.Name,
			ClusterHash:   l.Cluster.ClusterInfo.Hash,
			ClusterId:     l.Cluster.ClusterInfo.Id(),
			InputManifest: l.ProjectName,
			Type:          cluster_builder.LoadbalancerCluster,
			LBInfo: cluster_builder.LoadbalancerClusterInfo{
				Roles: l.Cluster.Roles,
			},
			SpawnProcessLimit: l.SpawnProcessLimit,
		}

		if err := builder.Init(logger, dynamic); err != nil {
			return err
		}
		defer builder.Done()

		out, err := builder.OutputOnlyCommon()
		if err != nil {
			return fmt.Errorf("failed to read output of common nodepool infrastrucutre: %w", err)
		}

		group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
		group.SetLimit(l.NodePoolConcurrencyLimit)

		for i, np := range dynamic {
			key := templates.StateFile(
				builder.InputManifest,
				cluster_builder.StateFileNodePoolSubKey(builder.ClusterId, np.Name),
			)
			group.Go(func() error {
				if err := s3.Stat(ctx, key); err != nil {
					if !errors.Is(err, store.ErrS3KeyNotExists) {
						return fmt.Errorf("failed to check presence of state file for cluster: %w", err)
					}

					logger.Warn().Msgf("No state file found for cluster, assuming infrastructure was deleted")
					return nil
				}

				if err := builder.DestroyNodePool(out, i); err != nil {
					return fmt.Errorf("failed to destroy nodepool %q: %w", np.Name, err)
				}

				if err := s3.DeleteStateFile(ctx, key); err != nil {
					return fmt.Errorf("failed to delete state file for nodepool %q: %w", np.Name, err)
				}
				return nil
			})

		}

		if err := group.Wait(); err != nil {
			return fmt.Errorf("failed to delete all nodepools: %w", err)
		}

		key := templates.StateFile(
			builder.InputManifest,
			cluster_builder.StateFileCommonInfrastructureSubKey(builder.ClusterId),
		)
		if err := s3.Stat(ctx, key); err != nil {
			if !errors.Is(err, store.ErrS3KeyNotExists) {
				return fmt.Errorf("failed to check presence of state file for common nodepool infrastructure: %w", err)
			}
			logger.Warn().Msg("No state file found for common nodepool infrastructure, assuming it was deleted")
			return nil
		}

		if err := builder.DestroyCommon(); err != nil {
			return fmt.Errorf("failed to destroy common nodepool infrastructure: %w", err)
		}

		if err := s3.DeleteStateFile(ctx, key); err != nil {
			return fmt.Errorf("failed to delete state file for common nodepool infrastructure: %w", err)
		}
		return nil
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

		builder := cluster_builder.DnsBuilder{
			ClusterName:       l.Cluster.ClusterInfo.Name,
			ClusterHash:       l.Cluster.ClusterInfo.Hash,
			ClusterId:         l.Cluster.ClusterInfo.Id(),
			InputManifest:     l.ProjectName,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}

		if err := builder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
			return err
		}
		defer builder.Done()

		key := templates.StateFile(builder.InputManifest, cluster_builder.StateFileDnsSubKey(builder.ClusterId))
		if err := s3.Stat(ctx, key); err != nil {
			if !errors.Is(err, store.ErrS3KeyNotExists) {
				return fmt.Errorf("failed to check presence of state file for dns: %w", err)
			}
			logger.Warn().Msg("No state file found for DNS, assuming it was deleted")
			return nil
		}

		if err := builder.DestroyRecords(); err != nil {
			return fmt.Errorf("failed to destroy dns records: %w", err)
		}

		if err := s3.DeleteStateFile(ctx, key); err != nil {
			return fmt.Errorf("failed to delete state file for dns: %w", err)
		}
		return nil
	})

	if err := group.Wait(); err != nil {
		return fmt.Errorf("failed to fully destroy loadbalancer: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, l.Cluster.ClusterInfo.Id())); err != nil {
		return fmt.Errorf("failed to delete external templates for cluster: %w", err)
	}
	return nil
}

// ReoncileNodePools reconciles the state of a single nodepool.
func (c *LBcluster) ReconcileNodePool(ctx context.Context, logger zerolog.Logger, handle int) error {
	panic("todo")
}

// DestroyNodePool destroys the state of a single nodepool.
func (c *LBcluster) DestroyNodePool(ctx context.Context, logger zerolog.Logger, handle int, s3 store.S3StateStorage) error {
	panic("todo")
}

// ReconcileCommon reconciles common infrastructure for all of the nodepools of the cluster.
func (c *LBcluster) ReconcileCommon(ctx context.Context, logger zerolog.Logger) error { panic("todo") }

// DestroyCommon destroys common infrastructure for all of the nodepools of the cluster.
func (c *LBcluster) DestroyCommon(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error {
	panic("todo")
}
