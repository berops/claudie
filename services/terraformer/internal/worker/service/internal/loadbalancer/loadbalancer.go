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
const DefaultNodePoolConcurrencyLimit = 20

// ReconcileNodePoolMode sets the mode for how to deal with the common infrastructure
// when reconciling a single nodepool.
type ReconcileNodePoolMode uint8

const (
	// ReconcileModeRead reads only the output of the common infrastructure.
	ReconcileModeRead ReconcileNodePoolMode = iota

	// ReconcileModeReadWrite reconciles the common infrastructure
	// with the nodes of the cluster and reads the output.
	ReconcileModeReadWrite
)

var (
	// ErrReconcileAll error is returned when reconciliation of the common nodepool infrastructure
	// was executed and therefore nodepools may need to be reconciled aswell by calling the [LBcluster.ReconcileAll]
	// function.
	ErrReconcileAll = errors.New("reconciliation of the whole cluster may be needed")
)

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

// Reconciles the nodepool and its state. It is expected that the nodepool with the passed in 'handle' is part of the [LBcluster.Cluster].
func (l *LBcluster) ReconcileNodePool(_ context.Context, logger zerolog.Logger, handle string, mode ReconcileNodePoolMode) error {
	dynamic := nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
	switch idx := nodepools.IndexByName(handle, dynamic); {
	case idx >= 0:
		lbuilder := cluster_builder.ClusterBuilder{
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

		if err := lbuilder.Init(logger, dynamic); err != nil {
			return err
		}
		defer lbuilder.Cleanup()

		switch mode {
		case ReconcileModeRead:
			latest, err := lbuilder.OutputOnlyCommon()
			if err != nil {
				return fmt.Errorf("failed to read latest common nodepool infrastructure output: %w", err)
			}
			if err := lbuilder.ReconcileNodePool(latest, idx); err != nil {
				return fmt.Errorf("failed to reconcile nodepool: %w", err)
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
		case ReconcileModeReadWrite:
			updated, err := lbuilder.ReconcileCommon()
			if err != nil {
				return fmt.Errorf("failed to reconcile common nodepool infrastructure: %w: %w", err, ErrReconcileAll)
			}
			if err := lbuilder.ReconcileNodePool(updated, idx); err != nil {
				return fmt.Errorf("failed to reconcile nodepool: %w: %w", err, ErrReconcileAll)
			}

			dbuilder := cluster_builder.DnsBuilder{
				ClusterName:       l.Cluster.ClusterInfo.Name,
				ClusterHash:       l.Cluster.ClusterInfo.Hash,
				ClusterId:         l.Cluster.ClusterInfo.Id(),
				InputManifest:     l.ProjectName,
				SpawnProcessLimit: l.SpawnProcessLimit,
			}

			nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools) // include all of the nodes.
			if err := dbuilder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
				return fmt.Errorf("%w: %w", err, ErrReconcileAll)
			}
			defer dbuilder.Cleanup()

			if err := dbuilder.ReconcileRecords(); err != nil {
				return fmt.Errorf("error while reconciling dns records for cluster %q: %w: %w", dbuilder.ClusterId, err, ErrReconcileAll)
			}
			return ErrReconcileAll
		default:
			return fmt.Errorf("unknown reconcile mode %v", mode)
		}
	default:
		builder := cluster_builder.DnsBuilder{
			ClusterName:       l.Cluster.ClusterInfo.Name,
			ClusterHash:       l.Cluster.ClusterInfo.Hash,
			ClusterId:         l.Cluster.ClusterInfo.Id(),
			InputManifest:     l.ProjectName,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}

		nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools) // include all of the nodes.
		if err := builder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
			return err
		}
		defer builder.Cleanup()

		if err := builder.ReconcileRecords(); err != nil {
			return fmt.Errorf("error while reconciling dns records for cluster %q: %w", builder.ClusterId, err)
		}
		return nil
	}
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

// DestroyNodePool destroys the passed in nodepool and its state. It is expected that the nodepool is no longer part of the [LBcluster.Cluster].
//
// The [ErrReconcileAll] error is returned when the reconciliation of the common nodepool infrastructure is executed to indicate
// that other nodepools may have had their state invalidated.
func (l *LBcluster) DestroyNodePool(ctx context.Context, logger zerolog.Logger, deleted *spec.NodePool, s3 store.S3StateStorage) error {
	switch {
	case deleted.GetStaticNodePool() != nil:
		builder := cluster_builder.DnsBuilder{
			ClusterName:       l.Cluster.ClusterInfo.Name,
			ClusterHash:       l.Cluster.ClusterInfo.Hash,
			ClusterId:         l.Cluster.ClusterInfo.Id(),
			InputManifest:     l.ProjectName,
			SpawnProcessLimit: l.SpawnProcessLimit,
		}
		nodeIPs := nodepools.PublicEndpoints(l.Cluster.ClusterInfo.NodePools) // include all of the nodes.
		if err := builder.Init(logger, nodeIPs, l.Cluster.Dns); err != nil {
			return err
		}
		defer builder.Cleanup()

		if err := builder.ReconcileRecords(); err != nil {
			return fmt.Errorf("error while reconciling dns records for cluster %q: %w", builder.ClusterId, err)
		}
		return nil
	case deleted.GetDynamicNodePool() != nil:
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

		var (
			statefileExists = true
			lbuilder        = cluster_builder.ClusterBuilder{
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
			nodepoolStateFileKey = store.ObjectKey(lbuilder.InputManifest, cluster_builder.NodePoolStateKey(lbuilder.ClusterId, deleted.Name))
		)

		if err := s3.Stat(ctx, nodepoolStateFileKey); err != nil {
			if !errors.Is(err, store.ErrS3KeyNotExists) {
				return fmt.Errorf("failed to check presence of state file for cluster: %w", err)
			}
			statefileExists = false
		}

		if !statefileExists {
			logger.Warn().Msgf("No state file found, assuming nodepool %q was deleted", deleted.Name)
		}

		dynamic := nodepools.Dynamic(l.Cluster.ClusterInfo.NodePools)
		dynamic = append(dynamic, deleted)

		if err := lbuilder.Init(logger, dynamic); err != nil {
			return cluster_builder.ExplainUnknownCommit(err, lbuilder.ClusterId)
		}

		if statefileExists {
			out, err := lbuilder.OutputOnlyCommon()
			if err != nil {
				lbuilder.Cleanup()
				return fmt.Errorf("failed to read output of common nodepool infrastructure: %w", err)
			}

			if err := lbuilder.DestroyNodePool(out, len(dynamic)-1); err != nil {
				lbuilder.Cleanup()
				return fmt.Errorf("failed to destroy nodepool %q: %w", deleted.Name, err)
			}
		}

		lbuilder.Cleanup()

		dynamic = dynamic[:len(dynamic)-1]
		if err := lbuilder.Init(logger, dynamic, deleted /* Include the deleted nodepool for its Provider */); err != nil {
			return err
		}
		defer lbuilder.Cleanup()

		if err := s3.DeleteStateFile(ctx, nodepoolStateFileKey); err != nil {
			return fmt.Errorf("failed to delete state file for nodepool %q: %w", deleted.Name, err)
		}

		// Needs to follow DeleteStateFile for having idempotent deletes.
		if _, err := lbuilder.ReconcileCommon(); err != nil {
			return fmt.Errorf("failed to reconcile common nodepool infrastructure after nodepool deletion: %w: %w", err, ErrReconcileAll)
		}
		return ErrReconcileAll
	default:
		// Reconcile nothing.
		return nil
	}
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
