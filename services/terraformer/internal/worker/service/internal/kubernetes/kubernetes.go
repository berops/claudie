package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

type K8Scluster struct {
	ProjectName string
	Cluster     *spec.K8Scluster

	// Signals whether to export port 6443 on the
	// control plane nodes of the cluster.
	// This value is passed down when generating
	// the terraform templates.
	ExportPort6443 bool

	// How many NodePools can be worked on concurrently.
	//
	// When left uninitialized [DefaultNodePoolConcurrencyLimit] is used as the default
	NodePoolConcurrencyLimit int

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (k *K8Scluster) Id() string         { return k.Cluster.ClusterInfo.Id() }
func (k *K8Scluster) IsKubernetes() bool { return true }

// ReconcileAll reconciles the whole kubernetes cluster with the latest up to date data.
func (k *K8Scluster) ReconcileAll(ctx context.Context, logger zerolog.Logger) error {
	logger.Info().Msgf("Reconciling K8S Cluster")

	if k.NodePoolConcurrencyLimit == 0 {
		k.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	var (
		dynamic = nodepools.Dynamic(k.Cluster.ClusterInfo.NodePools)
		builder = cluster_builder.ClusterBuilder{
			ClusterName:   k.Cluster.ClusterInfo.Name,
			ClusterHash:   k.Cluster.ClusterInfo.Hash,
			ClusterId:     k.Cluster.ClusterInfo.Id(),
			InputManifest: k.ProjectName,
			Type:          cluster_builder.KubernetesCluster,
			K8sInfo: cluster_builder.KubernetesClusterInfo{
				ExportPort6443: k.ExportPort6443,
			},
			SpawnProcessLimit: k.SpawnProcessLimit,
		}
	)

	if err := builder.Init(logger, dynamic); err != nil {
		return fmt.Errorf("error while reconciling kuberentes cluster %q: %w", builder.ClusterId, err)
	}
	defer builder.Cleanup()

	out, err := builder.ReconcileCommon()
	if err != nil {
		return fmt.Errorf("error while reconciling common nodepool infrastructure: %w", err)
	}

	group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
	group.SetLimit(k.NodePoolConcurrencyLimit)

	for i := range dynamic {
		group.Go(func() error { return builder.ReconcileNodePool(out, i) })
	}
	return group.Wait()
}

// DestroyAll destroys all of the infrastructure of the kubernetes cluster.
func (k *K8Scluster) DestroyAll(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error {
	logger.Info().Msgf("Destroying K8S Cluster")

	if k.NodePoolConcurrencyLimit == 0 {
		k.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	dynamic := nodepools.Dynamic(k.Cluster.ClusterInfo.NodePools)
	builder := cluster_builder.ClusterBuilder{
		ClusterName:   k.Cluster.ClusterInfo.Name,
		ClusterHash:   k.Cluster.ClusterInfo.Hash,
		ClusterId:     k.Cluster.ClusterInfo.Id(),
		InputManifest: k.ProjectName,
		Type:          cluster_builder.KubernetesCluster,
		K8sInfo: cluster_builder.KubernetesClusterInfo{
			ExportPort6443: k.ExportPort6443,
		},
		SpawnProcessLimit: k.SpawnProcessLimit,
	}

	if err := builder.Init(logger, dynamic); err != nil {
		return cluster_builder.ExplainUnknownCommit(err, builder.ClusterId)
	}
	defer builder.Cleanup()

	latest, err := builder.OutputOnlyCommon()
	if err != nil {
		return fmt.Errorf("failed to read output of common nodepool infrastructure: %w", err)
	}

	group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
	group.SetLimit(k.NodePoolConcurrencyLimit)

	for i, np := range dynamic {
		nodepoolStateFileKey := store.ObjectKey(builder.InputManifest, cluster_builder.NodePoolStateKey(builder.ClusterId, np.Name))
		group.Go(func() error {
			if err := s3.Stat(ctx, nodepoolStateFileKey); err != nil {
				if !errors.Is(err, store.ErrS3KeyNotExists) {
					return fmt.Errorf("failed to read statefile for cluster: %w", err)
				}

				logger.
					Warn().
					Msgf("No state file found for nodepool %q, assuming infrastructure was deleted", np.Name)

				return nil
			}

			if err := builder.DestroyNodePool(latest, i); err != nil {
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

	defer func() {
		if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, builder.ClusterId)); err != nil {
			logger.Err(err).Msg("failed to delete external templates directory")
		}
	}()

	commonInfraStateFileKey := store.ObjectKey(builder.InputManifest, cluster_builder.CommonInfraStateKey(builder.ClusterId))
	if err := s3.Stat(ctx, commonInfraStateFileKey); err != nil {
		if !errors.Is(err, store.ErrS3KeyNotExists) {
			return fmt.Errorf("failed to read statefile for common nodepool infrastructure: %w", err)
		}
		logger.Warn().Msgf("no state file found for common nodepool infrastructure, assuming it was deleted")
		return nil
	}

	if err := builder.DestroyCommon(); err != nil {
		return fmt.Errorf("failed to destroy common nodepool infrastructure: %w", err)
	}

	if err := s3.DeleteStateFile(ctx, commonInfraStateFileKey); err != nil {
		return fmt.Errorf("failed to delete state file for common nodepool infrastructure: %w", err)
	}

	return nil
}

// ReconcileCommon reconciles common infrastructure for all of the nodepools of the cluster.
func (c *K8Scluster) ReconcileCommon(_ context.Context, logger zerolog.Logger) error {
	builder := cluster_builder.ClusterBuilder{
		ClusterName:   c.Cluster.ClusterInfo.Name,
		ClusterHash:   c.Cluster.ClusterInfo.Hash,
		ClusterId:     c.Cluster.ClusterInfo.Id(),
		InputManifest: c.ProjectName,
		Type:          cluster_builder.KubernetesCluster,
		K8sInfo: cluster_builder.KubernetesClusterInfo{
			ExportPort6443: c.ExportPort6443,
		},
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	dynamic := nodepools.Dynamic(c.Cluster.ClusterInfo.NodePools)
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
func (c *K8Scluster) ReconcileCommonNoReplace(_ context.Context, logger zerolog.Logger, added, removed []*spec.NodePool) error {
	builder := cluster_builder.ClusterBuilder{
		ClusterName:   c.Cluster.ClusterInfo.Name,
		ClusterHash:   c.Cluster.ClusterInfo.Hash,
		ClusterId:     c.Cluster.ClusterInfo.Id(),
		InputManifest: c.ProjectName,
		Type:          cluster_builder.KubernetesCluster,
		K8sInfo: cluster_builder.KubernetesClusterInfo{
			ExportPort6443: c.ExportPort6443,
		},
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	dynamic := append(nodepools.Dynamic(c.Cluster.ClusterInfo.NodePools), nodepools.Dynamic(added)...)
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
// using the already existing common infrastructure, which is only read. Names that
// are not dynamic nodepools of the cluster are ignored.
func (c *K8Scluster) NodePools(ctx context.Context, logger zerolog.Logger, names ...string) error {
	if c.NodePoolConcurrencyLimit == 0 {
		c.NodePoolConcurrencyLimit = DefaultNodePoolConcurrencyLimit
	}

	var (
		dynamic = nodepools.Dynamic(c.Cluster.ClusterInfo.NodePools)
		builder = cluster_builder.ClusterBuilder{
			ClusterName:   c.Cluster.ClusterInfo.Name,
			ClusterHash:   c.Cluster.ClusterInfo.Hash,
			ClusterId:     c.Cluster.ClusterInfo.Id(),
			InputManifest: c.ProjectName,
			Type:          cluster_builder.KubernetesCluster,
			K8sInfo: cluster_builder.KubernetesClusterInfo{
				ExportPort6443: c.ExportPort6443,
			},
			SpawnProcessLimit: c.SpawnProcessLimit,
		}
		handles []int
	)

	for _, name := range names {
		if idx := nodepools.IndexByName(name, dynamic); idx >= 0 {
			handles = append(handles, idx)
		}
	}
	if len(handles) == 0 {
		return nil
	}

	if err := builder.Init(logger, dynamic); err != nil {
		return err
	}
	defer builder.Cleanup()

	out, err := builder.OutputOnlyCommon()
	if err != nil {
		return fmt.Errorf("failed to read common nodepool infrastructure: %w", err)
	}

	group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
	group.SetLimit(c.NodePoolConcurrencyLimit)

	for _, i := range handles {
		group.Go(func() error { return builder.ReconcileNodePool(out, i) })
	}
	return group.Wait()
}

// DestroyNodePoolOnly destroys the infrastructure of the passed in nodepool and deletes
// its state file, without reconciling the common infrastructure. It is expected that the
// nodepool is no longer part of the [K8Scluster.Cluster]. A missing state file is a no-op.
func (c *K8Scluster) DestroyNodePoolOnly(ctx context.Context, logger zerolog.Logger, deleted *spec.NodePool, s3 store.S3StateStorage) error {
	var (
		builder = cluster_builder.ClusterBuilder{
			ClusterName:   c.Cluster.ClusterInfo.Name,
			ClusterHash:   c.Cluster.ClusterInfo.Hash,
			ClusterId:     c.Cluster.ClusterInfo.Id(),
			InputManifest: c.ProjectName,
			Type:          cluster_builder.KubernetesCluster,
			K8sInfo: cluster_builder.KubernetesClusterInfo{
				ExportPort6443: c.ExportPort6443,
			},
			SpawnProcessLimit: c.SpawnProcessLimit,
		}
		nodepoolStateFileKey = store.ObjectKey(builder.InputManifest, cluster_builder.NodePoolStateKey(builder.ClusterId, deleted.Name))
	)

	if err := s3.Stat(ctx, nodepoolStateFileKey); err != nil {
		if !errors.Is(err, store.ErrS3KeyNotExists) {
			return fmt.Errorf("failed to read statefile for nodepool %q: %w", deleted.Name, err)
		}
		logger.Info().Msgf("No state file found for nodepool %q, assuming infrastructure was deleted", deleted.Name)
		return nil
	}

	dynamic := append(nodepools.Dynamic(c.Cluster.ClusterInfo.NodePools), deleted)
	if err := builder.Init(logger, dynamic); err != nil {
		return cluster_builder.ExplainUnknownCommit(err, builder.ClusterId)
	}
	defer builder.Cleanup()

	latest, err := builder.OutputOnlyCommon()
	if err != nil {
		return fmt.Errorf("failed to read last common nodepool infrastructure output: %w", err)
	}

	if err := builder.DestroyNodePool(latest, len(dynamic)-1); err != nil {
		return fmt.Errorf("failed to destroy nodepool %q: %w", deleted.Name, err)
	}

	if err := s3.DeleteStateFile(ctx, nodepoolStateFileKey); err != nil {
		return fmt.Errorf("failed to delete state file for nodepool %q: %w", deleted.Name, err)
	}
	return nil
}
