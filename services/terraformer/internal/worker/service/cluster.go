package service

import (
	"context"

	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"
)

type Cluster interface {
	// Reconciles all of the infrastructure of the cluster.
	ReconcileAll(ctx context.Context, logger zerolog.Logger) error

	// Destroys all of the infrastructure of the cluster.
	DestroyAll(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error

	// ReoncileNodePools reconciles the state of a single nodepool.
	ReconcileNodePool(ctx context.Context, logger zerolog.Logger, handle int) error

	// DestroyNodePool destroys the state of a single nodepool.
	DestroyNodePool(ctx context.Context, logger zerolog.Logger, handle int, s3 store.S3StateStorage) error

	// ReconcileCommon reconciles common infrastructure for all of the nodepools of the cluster.
	ReconcileCommon(ctx context.Context, logger zerolog.Logger) error

	// DestroyCommon destroys common infrastructure for all of the nodepools of the cluster.
	DestroyCommon(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error

	// Id returns a cluster ID for the cluster.
	Id() string

	// whether the cluster is a kubernetes cluster, if not it is a loadbalancer.
	IsKubernetes() bool
}
