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
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/templates"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// The default value on how many nodepools can be built concurrently.
const DefaultNodePoolConcurrencyLimit = 10

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

func (k *K8Scluster) ReconcileAll(ctx context.Context, logger zerolog.Logger) error {
	logger.Info().Msgf("Building K8S Cluster %s", k.Cluster.ClusterInfo.Name)

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
		return fmt.Errorf("error while reconciling kuberentes cluster %q: %w", builder.ClusterId, err)
	}
	defer builder.Done()

	out, err := builder.ReconcileCommon()
	if err != nil {
		return fmt.Errorf("error while reconciling common nodepool infrastrucutre for %q: %w", builder.ClusterId, err)
	}

	group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
	group.SetLimit(k.NodePoolConcurrencyLimit)

	for i := range dynamic {
		group.Go(func() error { return builder.ReconcileNodePool(out, i) })
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("error while reconciling nodepools for cluster %q: %w", builder.ClusterId, err)
	}

	return nil
}

func (k *K8Scluster) DestroyAll(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error {
	logger.Info().Msgf("Destroying K8S Cluster %s", k.Cluster.ClusterInfo.Name)

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
		return err
	}
	defer builder.Done()

	out, err := builder.OutputOnlyCommon()
	if err != nil {
		return fmt.Errorf("failed to read output of common nodepool infrastructure: %w", err)
	}

	group, _ /* ctx Currently not used */ := errgroup.WithContext(ctx)
	group.SetLimit(k.NodePoolConcurrencyLimit)

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
		logger.Warn().Msgf("no state file found for common nodepool infrastructure, assuming it was deleted")
		return nil
	}

	if err := builder.DestroyCommon(); err != nil {
		return fmt.Errorf("failed to destroy common nodepool infrastructure: %w", err)
	}

	if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, builder.ClusterId)); err != nil {
		return fmt.Errorf("failed to delete external templates for cluster: %w", err)
	}

	if err := s3.DeleteStateFile(ctx, key); err != nil {
		return fmt.Errorf("failed to delete state file for common nodepool infrastructure: %w", err)
	}
	return nil
}

// ReoncileNodePools reconciles the state of a single nodepool.
func (c *K8Scluster) ReconcileNodePool(ctx context.Context, logger zerolog.Logger, handle int) error {
	panic("todo")
}

// DestroyNodePool destroys the state of a single nodepool.
func (c *K8Scluster) DestroyNodePool(ctx context.Context, logger zerolog.Logger, handle int, s3 store.S3StateStorage) error {
	panic("todo")
}

// ReconcileCommon reconciles common infrastructure for all of the nodepools of the cluster.
func (c *K8Scluster) ReconcileCommon(ctx context.Context, logger zerolog.Logger) error { panic("todo") }

// DestroyCommon destroys common infrastructure for all of the nodepools of the cluster.
func (c *K8Scluster) DestroyCommon(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error {
	panic("todo")
}
