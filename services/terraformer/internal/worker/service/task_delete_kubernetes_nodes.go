package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type DeleteKubernetesNodes struct {
	State  *spec.Update_State
	Delete *spec.Update_DeleteK8SNodes
}

func deleteKubernetesNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteKubernetesNodes,
	tracker Tracker,
) {
	// Currently there is no special mechanism for just deleting the
	// nodes of the kubernetes cluster, thus simply just remove them
	// from the state and reconcile the cluster, as there is just one
	// state file for the whole cluster.

	k8s := action.State.K8S

	if action.Delete.WithNodePool {
		k8s.ClusterInfo.NodePools = nodepools.DeleteByName(k8s.ClusterInfo.NodePools, action.Delete.Nodepool)
	} else {
		np := nodepools.FindByName(action.Delete.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't delete nodes from nodepool %q of kubernetes cluster %q as the nodepool is missing form the received state",
					action.Delete.Nodepool,
					k8s.ClusterInfo.Id(),
				)
			return
		}
		nodepools.DeleteNodes(np, action.Delete.Nodes)
	}

	cluster := kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		ExportPort6443:    clusters.FindAssignedLbApiEndpoint(action.State.LoadBalancers) == nil,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
	if err := BuildK8Scluster(buildLogger, cluster); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster after node deletion")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}
