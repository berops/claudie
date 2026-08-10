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
	// was executed and therefore nodepools may need to be reconciled aswell by calling the [K8Scluster.ReconcileAll]
	// function.
	ErrReconcileAll = errors.New("reconciliation of the whole cluster may be needed")
)

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

// ReoncileNodePool reconciles the state of a single nodepool. It is expected that the nodepool with the passed in 'handle' is part
// of the [K8Scluster.Cluster].
//
// When the reconciliation of the common nodepool infrastructure is performed the [ErrReconcileAll] error is returned hinting that all
// of the infrastructure should be reconciled via the [K8Scluster.ReconcileAll].
func (c *K8Scluster) ReconcileNodePool(_ context.Context, logger zerolog.Logger, handle string, mode ReconcileNodePoolMode) error {
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
	)

	idx := nodepools.IndexByName(handle, dynamic)
	if idx < 0 {
		return nil
	}

	err := builder.Init(logger, dynamic)
	if err != nil {
		return err
	}
	defer builder.Cleanup()

	switch mode {
	case ReconcileModeRead:
		latest, err := builder.OutputOnlyCommon()
		if err != nil {
			return fmt.Errorf("failed to reconcile common nodepool infrastructure: %w", err)
		}
		return builder.ReconcileNodePool(latest, idx)
	case ReconcileModeReadWrite:
		updated, err := builder.ReconcileCommon()
		if err != nil {
			return fmt.Errorf("failed to reconcile common nodepool infrastructure: %w: %w", err, ErrReconcileAll)
		}
		if err := builder.ReconcileNodePool(updated, idx); err != nil {
			return fmt.Errorf("failed to reconcile nodepool: %w: %w", err, ErrReconcileAll)
		}
		return ErrReconcileAll
	default:
		return fmt.Errorf("unknown nodepool reconcile mode %d", mode)
	}
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
		return err
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

	if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, builder.ClusterId)); err != nil {
		return fmt.Errorf("failed to delete external templates for cluster: %w", err)
	}

	if err := s3.DeleteStateFile(ctx, commonInfraStateFileKey); err != nil {
		return fmt.Errorf("failed to delete state file for common nodepool infrastructure: %w", err)
	}

	return nil
}

// DestroyNodePool destroys the passed in nodepool and its state. It is expected that the nodepool is no longer part of the [K8Scluster.Cluster].
//
// The [ErrReconcileAll] error is returned when the reconciliation of the common nodepool infrastructure is executed to indicate
// that other nodepools may have had their state invalidated.
func (c *K8Scluster) DestroyNodePool(ctx context.Context, logger zerolog.Logger, deleted *spec.NodePool, s3 store.S3StateStorage) error {
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

	dynamic := nodepools.Dynamic(c.Cluster.ClusterInfo.NodePools)
	dynamic = append(dynamic, deleted)

	if err := builder.Init(logger, dynamic); err != nil {
		return err
	}

	statefileExists := true
	if err := s3.Stat(ctx, nodepoolStateFileKey); err != nil {
		if !errors.Is(err, store.ErrS3KeyNotExists) {
			builder.Cleanup()
			return fmt.Errorf("failed to read statefile for nodepool %q: %w", deleted.Name, err)
		}

		logger.
			Warn().
			Msgf("No state file found for nodepool %q, assuming infrastructure was deleted", deleted.Name)

		statefileExists = false
	}

	if statefileExists {
		latest, err := builder.OutputOnlyCommon()
		if err != nil {
			builder.Cleanup()
			return fmt.Errorf("failed to read last common nodepool infrastructure output: %w", err)
		}

		if err := builder.DestroyNodePool(latest, len(dynamic)-1); err != nil {
			builder.Cleanup()
			return fmt.Errorf("failed to destroy nodepool %q: %w", deleted.Name, err)
		}
	}

	builder.Cleanup()

	dynamic = dynamic[:len(dynamic)-1]
	if err := builder.Init(logger, dynamic, deleted /* Include the deleted nodepool for its provider */); err != nil {
		return err
	}
	defer builder.Cleanup()

	if err := s3.DeleteStateFile(ctx, nodepoolStateFileKey); err != nil {
		return fmt.Errorf("failed to delete state file for nodepool %q: %w", deleted.Name, err)
	}

	// Needs to follow DeleteStateFile for having idempotent deletes.
	if _, err := builder.ReconcileCommon(); err != nil {
		return fmt.Errorf("failed to reconcile common nodepool infrastructure after nodepool deletion: %w: %w", err, ErrReconcileAll)
	}
	return ErrReconcileAll
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
