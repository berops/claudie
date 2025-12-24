package service

import (
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
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteKubernetesNodes,
	tracker Tracker,
) {
	// The deletion of the nodes for the kubernetes cluster is handled by the
	// kuber service, in here we only destroy the spawned infrastructure for the
	// dynamic nodepools.
	//
	// The state has already been modified and does not include the deleted nodes
	// thus simply refresh the state file with opentofu, as we currently share a
	// single state file within the cluster, which will take care of the deletions
	// of the infrastructure.

	k8s := action.State.K8S
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
