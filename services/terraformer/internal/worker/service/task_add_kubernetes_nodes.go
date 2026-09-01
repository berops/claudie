package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type AddKubernetesNodes struct {
	State *spec.Update_State
	Add   *spec.Update_TerraformerAddK8SNodes
}

func addKubernetesNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action AddKubernetesNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Adding kubernetes Nodes")

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

	switch kind := action.Add.Kind.(type) {
	case *spec.Update_TerraformerAddK8SNodes_Existing_:
		np := nodepools.FindByName(kind.Existing.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't add nodes to nodepool %q of kubernetes cluster %q as the nodepool is missing from the received state",
					kind.Existing.Nodepool,
					k8s.ClusterInfo.Id(),
				)
			return
		}

		if np.GetStaticNodePool() != nil {
			// Static nodes are not, and should be not, added through the
			// terraformer stage, thus here we can only focus on considering that
			// the nodes to be added here are dynamic nodes.
			logger.
				Warn().
				Msgf(
					"Can't work with static nodes from nodepool %q within kubernetes cluster %q, as their infrastructure cannot be managed by claudie, ignoring",
					np.Name,
					k8s.ClusterInfo.Id(),
				)
			return
		}

		nodepools.AppendDynamicNodes(np, kind.Existing.Nodes)
		if err := cluster.ReconcileNodePool(ctx, logger, kind.Existing.Nodepool, kubernetes.ReconcileModeRead); err != nil {
			if errors.Is(err, kubernetes.ErrReconcileAll) {
				err = cluster.ReconcileAll(ctx, logger)
			}

			if err != nil {
				logger.Err(err).Msgf("Failed to reconcile new nodes for nodepool %q", kind.Existing.Nodepool)
				tracker.Diagnostics.Push(err)
				// since there currently is no mechanism for tracking partial changes out
				// of the terraform output, commit the changes and let manager work out the diff.
				//
				// fallthrough
			}
		}
	case *spec.Update_TerraformerAddK8SNodes_New_:
		k8s.ClusterInfo.NodePools = append(k8s.ClusterInfo.NodePools, kind.New.Nodepool)
		if err := cluster.ReconcileNodePool(ctx, logger, kind.New.Nodepool.Name, kubernetes.ReconcileModeReadWrite); err != nil {
			if errors.Is(err, kubernetes.ErrReconcileAll) {
				err = cluster.ReconcileAll(ctx, logger)
			}

			if err != nil {
				logger.Err(err).Msgf("Failed to reconcile new nodepool %q", kind.New.Nodepool.Name)
				tracker.Diagnostics.Push(err)
				// since there currently is no mechanism for tracking partial changes out
				// of the terraform output, commit the changes and let manager work out the diff.
				//
				// fallthrough
			}
		}
	default:
		logger.
			Warn().
			Msgf("Received add nodes to kuberentes action, but with an invalid addition kind %T, ignoring", kind)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}
