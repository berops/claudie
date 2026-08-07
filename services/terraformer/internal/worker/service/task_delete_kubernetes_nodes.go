package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type DeleteKubernetesNodes struct {
	State  *spec.Update_State
	Delete *spec.Update_DeletedK8SNodes
}

func deleteKubernetesNodes(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteKubernetesNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Deleting Kubernetes Nodes")

	// The deletion of the nodes for the kubernetes cluster is handled by the
	// kuber service, in here we only destroy the spawned infrastructure for the
	// dynamic nodepools.
	//
	// The state has already been modified and does not include the deleted nodes
	// thus simply refresh the state file with opentofu, as we currently share a
	// single state file within the cluster, which will take care of the deletions
	// of the infrastructure.

	var (
		ctx     = context.Background()
		k8s     = action.State.K8S
		cluster = kubernetes.K8Scluster{
			ProjectName:       projectName,
			Cluster:           k8s,
			ExportPort6443:    clusters.FindAssignedLbApiEndpoint(action.State.LoadBalancers) == nil,
			SpawnProcessLimit: processLimit,
		}
	)

	logger = logger.With().Str("cluster", cluster.Id()).Logger()

	switch kind := action.Delete.Kind.(type) {
	case *spec.Update_DeletedK8SNodes_Partial_:
		if err := cluster.ReconcileNodePool(ctx, logger, kind.Partial.Nodepool, kubernetes.ReconcileModeRead); err != nil {
			if !errors.Is(err, kubernetes.ErrReconcileAll) {
				logger.Err(err).Msgf("Failed to destroy infrastructure for removed nodes of nodepool %q", kind.Partial.Nodepool)
				tracker.Diagnostics.Push(err)
				return
			}

			logger.
				Warn().
				Msgf("Handling Reconcile All, after nodes deletion from nodepool %q", kind.Partial.Nodepool)

			if err := cluster.ReconcileAll(ctx, logger); err != nil {
				logger.Err(err).Msgf("Failed to destroy infrastructure for removed nodes of nodepool %q", kind.Partial.Nodepool)
				tracker.Diagnostics.Push(err)
				return
			}
		}
	case *spec.Update_DeletedK8SNodes_Whole:
		if kind.Whole.Nodepool.GetDynamicNodePool() == nil {
			// Nothing to delete for static nodepools.
			return
		}

		if err := cluster.DestroyNodePool(ctx, logger, kind.Whole.Nodepool, stores.s3); err != nil {
			if !errors.Is(err, kubernetes.ErrReconcileAll) {
				logger.Err(err).Msgf("Failed to destroy nodepool %q", kind.Whole.Nodepool.Name)
				tracker.Diagnostics.Push(err)
				return

			}

			logger.
				Warn().
				Msgf("Handling Reconcile All, after %q nodepool deletion", kind.Whole.Nodepool.Name)

			if err := cluster.ReconcileAll(ctx, logger); err != nil {
				logger.Err(err).Msgf("Failed reconcile infra after nodepool %q destruction", kind.Whole.Nodepool.Name)
				tracker.Diagnostics.Push(err)
				return
			}
		}
	default:
		logger.
			Warn().
			Msgf("Received delete nodes for kuberentes, but with an invalid kind %T, ignoring", kind)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}
