package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// The default value on how many nodepools can be built concurrently.
const DefaultNodePoolConcurrencyLimit = 8

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

	var (
		dynamic = nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
		builder = cluster_builder.ClusterBuilder{
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
		dnsBuilder = cluster_builder.DnsBuilder{
			ClusterName:       l.Cluster.ClusterInfo.Name,
			ClusterHash:       l.Cluster.ClusterInfo.Hash,
			ClusterId:         l.Cluster.ClusterInfo.Id(),
			InputManifest:     l.ProjectName,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}
	)

	if err := builder.Init(logger, dynamic); err != nil {
		return err
	}
	defer builder.Cleanup()

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
	defer dnsBuilder.Cleanup()

	if err := dnsBuilder.ReconcileRecords(); err != nil {
		return fmt.Errorf("error while reconciling dns records for cluster %q: %w", builder.ClusterId, err)
	}
	return nil
}

// DestroyAll destroys all of the infrastructure of the loadbalancer cluster.
func (l *LBcluster) DestroyAll(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error {
	logger.Info().Msg("Destroying LB Cluster and its DNS records")

	if l.NodePoolConcurrencyLimit == 0 {
		l.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	var (
		nodeIPs  = nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools)
		dynamic  = nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
		lbuilder = cluster_builder.ClusterBuilder{
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
		dbuilder = cluster_builder.DnsBuilder{
			ClusterName:       l.Cluster.ClusterInfo.Name,
			ClusterHash:       l.Cluster.ClusterInfo.Hash,
			ClusterId:         l.Cluster.ClusterInfo.Id(),
			InputManifest:     l.ProjectName,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}
	)

	// Exclude nodes with no IP's, as those are nodes that were not successfully built.
	nodeIPs = slices.DeleteFunc(nodeIPs, func(s string) bool { return s == "" })

	if err := lbuilder.Init(logger, dynamic); err != nil {
		return cluster_builder.ExplainUnknownCommit(err, lbuilder.ClusterId)
	}
	defer lbuilder.Cleanup()

	group, ctx := errgroup.WithContext(ctx)

	if l.Cluster.Dns != nil {
		if err := dbuilder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
			return cluster_builder.ExplainUnknownCommit(err, dbuilder.ClusterId)
		}
		defer dbuilder.Cleanup()

		group.Go(func() error {
			key := store.ObjectKey(dbuilder.InputManifest, cluster_builder.DnsStateKey(dbuilder.ClusterId))
			if err := s3.Stat(ctx, key); err != nil {
				if !errors.Is(err, store.ErrS3KeyNotExists) {
					return fmt.Errorf("failed to check presence of state file for dns: %w", err)
				}
				logger.Warn().Msg("No state file found for DNS, assuming it was deleted")
				return nil
			}

			if err := dbuilder.DestroyRecords(); err != nil {
				return fmt.Errorf("failed to destroy dns records: %w", err)
			}

			if err := s3.DeleteStateFile(ctx, key); err != nil {
				return fmt.Errorf("failed to delete state file for dns: %w", err)
			}
			return nil
		})
	}

	group.Go(func() error {
		latest, err := lbuilder.OutputOnlyCommon()
		if err != nil {
			return fmt.Errorf("failed to read output of common nodepool infrastructure: %w", err)
		}

		group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
		group.SetLimit(l.NodePoolConcurrencyLimit)

		for i, np := range dynamic {
			nodepoolStateFileKey := store.ObjectKey(lbuilder.InputManifest, cluster_builder.NodePoolStateKey(lbuilder.ClusterId, np.Name))
			group.Go(func() error {
				if err := s3.Stat(ctx, nodepoolStateFileKey); err != nil {
					if !errors.Is(err, store.ErrS3KeyNotExists) {
						return fmt.Errorf("failed to check presence of state file for cluster: %w", err)
					}

					logger.
						Warn().
						Msgf("No state file found for nodepool %q, assuming infrastructure was deleted", np.Name)

					return nil
				}

				if err := lbuilder.DestroyNodePool(latest, i); err != nil {
					return fmt.Errorf("failed to destroy nodepool %q: %w", np.Name, err)
				}

				if err := s3.DeleteStateFile(ctx, nodepoolStateFileKey); err != nil {
					return fmt.Errorf("failed to delete state file for nodepool %q: %w", np.Name, err)
				}
				return nil
			})
		}

		if err := group.Wait(); err != nil {
			return fmt.Errorf("failed to delete all nodepools: %w", err)
		}

		commonInfraStateFileKey := store.ObjectKey(lbuilder.InputManifest, cluster_builder.CommonInfraStateKey(lbuilder.ClusterId))
		if err := s3.Stat(ctx, commonInfraStateFileKey); err != nil {
			if !errors.Is(err, store.ErrS3KeyNotExists) {
				return fmt.Errorf("failed to check presence of state file for common nodepool infrastructure: %w", err)
			}
			logger.Warn().Msg("No state file found for common nodepool infrastructure, assuming it was deleted")
			return nil
		}

		if err := lbuilder.DestroyCommon(); err != nil {
			return fmt.Errorf("failed to destroy common nodepool infrastructure: %w", err)
		}

		if err := s3.DeleteStateFile(ctx, commonInfraStateFileKey); err != nil {
			return fmt.Errorf("failed to delete state file for common nodepool infrastructure: %w", err)
		}
		return nil
	})

	defer func() {
		if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, l.Cluster.ClusterInfo.Id())); err != nil {
			logger.Err(err).Msg("failed to delete external templates directory")
		}
	}()

	if err := group.Wait(); err != nil {
		return fmt.Errorf("failed to fully destroy loadbalancer: %w", err)
	}
	return nil
}

// ReconcileCommon reconciles common infrastructure for all of the nodepools of the cluster.
func (l *LBcluster) ReconcileCommon(_ context.Context, logger zerolog.Logger) error {
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

	dynamic := nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
	if err := builder.Init(logger, dynamic); err != nil {
		return err
	}
	defer builder.Cleanup()

	if _, err := builder.ReconcileCommon(); err != nil {
		return fmt.Errorf("failed to reconcile common nodepool infrastructure: %w", err)
	}
	return nil
}

// ReconcileCommonNoReplace reconciles common infrastructure for all of the nodepools of
// the cluster and the added nodepools, which are not yet part of the cluster, refusing to
// replace existing resources, see [cluster_builder.ClusterBuilder.ReconcileCommonNoReplace].
// The removed nodepools, which must no longer be part of the cluster, are only used for
// generating their providers so that common infrastructure no longer used can be destroyed.
func (l *LBcluster) ReconcileCommonNoReplace(_ context.Context, logger zerolog.Logger, added, removed []*spec.NodePool) error {
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

	dynamic := append(nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools), nodepools.Dynamic(added)...)
	if err := builder.Init(logger, dynamic, removed...); err != nil {
		return err
	}
	defer builder.Cleanup()

	if _, err := builder.ReconcileCommonNoReplace(); err != nil {
		return fmt.Errorf("failed to reconcile common nodepool infrastructure: %w", err)
	}
	return nil
}

// NodePools reconciles the state of the dynamic nodepools with the passed in names,
// using the already existing common infrastructure, which is only read, followed by
// the reconciliation of the DNS records of the loadbalancer. Names that are not dynamic
// nodepools of the loadbalancer, i.e. static nodepools, only result in the reconciliation
// of the DNS records.
func (l *LBcluster) NodePools(ctx context.Context, logger zerolog.Logger, names ...string) error {
	if l.NodePoolConcurrencyLimit == 0 {
		l.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	dynamic := nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)

	var handles []int
	for _, name := range names {
		if idx := nodepools.IndexByName(name, dynamic); idx >= 0 {
			handles = append(handles, idx)
		}
	}

	if len(handles) > 0 {
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
		defer builder.Cleanup()

		out, err := builder.OutputOnlyCommon()
		if err != nil {
			return fmt.Errorf("failed to read latest common nodepool infrastructure output: %w", err)
		}

		group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
		group.SetLimit(l.NodePoolConcurrencyLimit)

		for _, i := range handles {
			group.Go(func() error { return builder.ReconcileNodePool(out, i) })
		}

		if err := group.Wait(); err != nil {
			return fmt.Errorf("failed to reconcile nodepools: %w", err)
		}
	}

	dbuilder := cluster_builder.DnsBuilder{
		ClusterName:       l.Cluster.ClusterInfo.Name,
		ClusterHash:       l.Cluster.ClusterInfo.Hash,
		ClusterId:         l.Cluster.ClusterInfo.Id(),
		InputManifest:     l.ProjectName,
		SpawnProcessLimit: l.SpawnProcessLimit,
	}

	// include the current nodes of the cluster, which now will also
	// either contain the new nodes or no longer contain the removed
	// nodes.
	nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools)
	if err := dbuilder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
		return err
	}
	defer dbuilder.Cleanup()

	if err := dbuilder.ReconcileRecords(); err != nil {
		return fmt.Errorf("error while reconciling dns records for cluster %q: %w", dbuilder.ClusterId, err)
	}
	return nil
}

// DestroyNodePoolOnly removes the nodes of the passed in nodepool from the DNS records
// of the loadbalancer and, for dynamic nodepools, destroys the infrastructure of the
// nodepool and deletes its state file, without reconciling the common infrastructure.
// It is expected that the nodepool is no longer part of the [LBcluster.Cluster]. A
// missing state file only reconciles the DNS records.
func (l *LBcluster) DestroyNodePoolOnly(ctx context.Context, logger zerolog.Logger, deleted *spec.NodePool, s3 store.S3StateStorage) error {
	dbuilder := cluster_builder.DnsBuilder{
		ClusterName:       l.Cluster.ClusterInfo.Name,
		ClusterHash:       l.Cluster.ClusterInfo.Hash,
		ClusterId:         l.Cluster.ClusterInfo.Id(),
		InputManifest:     l.ProjectName,
		SpawnProcessLimit: l.SpawnProcessLimit,
	}

	nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools) // include the current nodes of the cluster.
	if err := dbuilder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
		return cluster_builder.ExplainUnknownCommit(err, dbuilder.ClusterId)
	}
	defer dbuilder.Cleanup()

	if err := dbuilder.ReconcileRecords(); err != nil {
		return fmt.Errorf("error while reconciling dns records for cluster %q: %w", dbuilder.ClusterId, err)
	}

	if deleted.GetDynamicNodePool() == nil {
		return nil
	}

	var (
		builder = cluster_builder.ClusterBuilder{
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
		nodepoolStateFileKey = store.ObjectKey(builder.InputManifest, cluster_builder.NodePoolStateKey(builder.ClusterId, deleted.Name))
	)

	if err := s3.Stat(ctx, nodepoolStateFileKey); err != nil {
		if !errors.Is(err, store.ErrS3KeyNotExists) {
			return fmt.Errorf("failed to check presence of state file for cluster: %w", err)
		}
		logger.Info().Msgf("No state file found, assuming nodepool %q was deleted", deleted.Name)
		return nil
	}

	dynamic := append(nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools), deleted)
	if err := builder.Init(logger, dynamic); err != nil {
		return cluster_builder.ExplainUnknownCommit(err, builder.ClusterId)
	}
	defer builder.Cleanup()

	out, err := builder.OutputOnlyCommon()
	if err != nil {
		return fmt.Errorf("failed to read output of common nodepool infrastructure: %w", err)
	}

	if err := builder.DestroyNodePool(out, len(dynamic)-1); err != nil {
		return fmt.Errorf("failed to destroy nodepool %q: %w", deleted.Name, err)
	}

	if err := s3.DeleteStateFile(ctx, nodepoolStateFileKey); err != nil {
		return fmt.Errorf("failed to delete state file for nodepool %q: %w", deleted.Name, err)
	}
	return nil
}
