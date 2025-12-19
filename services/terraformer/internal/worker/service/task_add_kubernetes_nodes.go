package service

import (
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
	// Currently there is no special mechanism for adding the
	// nodes of the kubernetes cluster as the whole cluster
	// shares a single state file, thus simply just add the
	// new nodes to the state and reconcile the cluster.
	k8s := action.State.K8S

	switch kind := action.Add.Kind.(type) {
	case *spec.Update_TerraformerAddK8SNodes_Existing_:
		np := nodepools.FindByName(kind.Existing.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't add nodes to nodepool %q of kubernetes cluster %q as the nodepool is missing form the received state",
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

		nodepools.DynamicAddNodes(np, kind.Existing.Nodes)
	case *spec.Update_TerraformerAddK8SNodes_New_:
		k8s.ClusterInfo.NodePools = append(k8s.ClusterInfo.NodePools, kind.New.Nodepool)
	default:
		logger.
			Warn().
			Msgf("Received add nodes to kuberentes action, but with an invalid addition kind %T, ignoring", kind)
		return
	}

	cluster := kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		ExportPort6443:    clusters.FindAssignedLbApiEndpoint(action.State.LoadBalancers) == nil,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
	if err := BuildK8Scluster(buildLogger, cluster); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster after node addition")
		tracker.Diagnostics.Push(err)
		// Contrary to the deletion process, during the addition if any partial changes
		// take effect we have to report them back, however since there is currently
		// no mechanism for tracking partial changes out of the terraform output
		// commit the whole changes, and let manager work out the diff.
		//
		// fallthrough
	}

	update := tracker.Result.Update()
	update.Kubernetes(cluster.Cluster)
	update.Commit()
}
