package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
	"github.com/rs/zerolog"
)

func (u *Usecases) patchConfigMapsWithNewApiEndpoint(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	tasks := []Task{
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.PatchClusterInfoConfigMap(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching cluster-info config map",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.PatchKubeProxyConfigMap(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching kube-proxy config map",
		},
	}

	return u.processTasks(ctx, work, logger, tasks)
}

func (u *Usecases) patchKubeadmAndUpdateCilium(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	tasks := []Task{
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				var lbApiEndpoint string
				if ep := clusters.FindAssignedLbApiEndpoint(work.DesiredLoadbalancers); ep != nil {
					lbApiEndpoint = ep.Dns.Endpoint
				}
				return u.Kuber.PatchKubeadmConfigMap(work, lbApiEndpoint, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching kubeadm config map",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.CiliumRolloutRestart(work.DesiredCluster, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "restarting cilium daemon set",
		},
	}

	return u.processTasks(ctx, work, logger, tasks)
}

// reconcileK8sConfiguration reconciles desired k8s cluster configuration via kuber.
func (u *Usecases) reconcileK8sConfiguration(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	tasks := []Task{
		{
			do:          u.patchConfigMapsWithNewApiEndpoint,
			stage:       spec.Workflow_KUBER,
			description: "patching config maps with new k8s api endpoint",
			condition: func(work *builder.Context) bool {
				// Only patch ConfigMaps if kubeconfig changed.
				return work.CurrentCluster != nil && (work.CurrentCluster.Kubeconfig != work.DesiredCluster.Kubeconfig)
			},
		},
		{
			do:          u.patchKubeadmAndUpdateCilium,
			stage:       spec.Workflow_KUBER,
			description: "patching kubeadm and restarting cilium",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.RemoveLBScrapeConfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "removing loadbalancer scrape config",
			condition: func(work *builder.Context) bool {
				// If previous cluster had loadbalancers, and the new one does not, the old scrape config will be removed.
				return len(work.DesiredLoadbalancers) == 0 && len(work.CurrentLoadbalancers) > 0
			},
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreLBScrapeConfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "storing loadbalancer scrape config",
			condition: func(work *builder.Context) bool {
				// Create a scrape-config if there are loadbalancers in the new/updated cluster.
				return len(work.DesiredLoadbalancers) > 0
			},
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				resStorage, err := u.Kuber.SetUpStorage(work, u.Kuber.GetClient())
				if err != nil {
					return err
				}
				work.DesiredCluster = resStorage.DesiredCluster
				return nil
			},
			stage:       spec.Workflow_KUBER,
			description: "setting up longhorn for storage",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreKubeconfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "creating kubeconfig secret",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreClusterMetadata(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "creating cluster metadata secret",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.PatchNodes(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching k8s nodes",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.SetUpClusterAutoscaler(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deploying cluster autoscaler",
			condition: func(work *builder.Context) bool {
				// Set up Autoscaler if desired state is autoscaled.
				return work.DesiredCluster.AnyAutoscaledNodePools()
			},
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DestroyClusterAutoscaler(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting cluster autoscaler",
			condition: func(work *builder.Context) bool {
				// Destroy Autoscaler if current state is autoscaled, but desired is not.
				return work.CurrentCluster.AnyAutoscaledNodePools() && !work.DesiredCluster.AnyAutoscaledNodePools()
			},
		},
	}

	return u.processTasks(ctx, work, logger, tasks)
}

// deleteClusterData deletes the kubeconfig, cluster metadata and cluster autoscaler from management cluster.
func (u *Usecases) deleteClusterData(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	if work.CurrentCluster == nil {
		return nil
	}

	tasks := []Task{
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DeleteKubeconfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting kubeconfig secret",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DeleteClusterMetadata(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting cluster metadata secret",
		},
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DestroyClusterAutoscaler(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting cluster autoscaler",
			condition:   func(work *builder.Context) bool { return work.CurrentCluster.AnyAutoscaledNodePools() },
		},
	}

	return u.processTasks(ctx, work, logger, tasks)
}

func (u *Usecases) deleteNodesFromCurrentState(nodepools map[string]*spec.DeletedNodes, staticCount, dynamicCount int) Task {
	return Task{
		do: func(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
			resDelete, err := u.Kuber.DeleteNodes(work.CurrentCluster, nodepools, u.Kuber.GetClient())
			if err != nil {
				return err
			}
			work.CurrentCluster = resDelete.Cluster
			return nil
		},
		stage:       spec.Workflow_KUBER,
		description: fmt.Sprintf("deleting nodes from cluster static: %v, dynamic: %v", staticCount, dynamicCount),
	}
}
