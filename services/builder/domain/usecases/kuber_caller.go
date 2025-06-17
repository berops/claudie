package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
	"github.com/rs/zerolog"
)

func (u *Usecases) patchConfigMapsWithNewApiEndpoint(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	return u.processTasks(ctx, work, logger, []Task{
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.PatchClusterInfoConfigMap(bx, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching cluster-info config map",
		},
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.PatchKubeProxyConfigMap(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching kube-proxy config map",
		},
	})
}

func (u *Usecases) patchKubeadmAndUpdateCilium(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	return u.processTasks(ctx, work, logger, []Task{
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
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
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.CiliumRolloutRestart(work.DesiredCluster, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "restarting cilium daemon set",
		},
	})
}

// reconcileK8sConfiguration reconciles desired k8s cluster configuration via kuber.
func (u *Usecases) reconcileK8sConfiguration(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	var tasks []Task

	if work.CurrentCluster != nil && (work.CurrentCluster.Kubeconfig != work.DesiredCluster.Kubeconfig) {
		// Only patch ConfigMaps if kubeconfig changed.
		tasks = append(tasks, Task{
			do:          u.patchConfigMapsWithNewApiEndpoint,
			stage:       spec.Workflow_KUBER,
			description: "patching configs maps with new k8s api endpoint",
		})
	}

	tasks = append(tasks, Task{
		do:          u.patchKubeadmAndUpdateCilium,
		stage:       spec.Workflow_KUBER,
		description: "patching kubeamd and restating cilium",
	})

	if len(work.DesiredLoadbalancers) == 0 && len(work.CurrentLoadbalancers) > 0 {
		// If previous cluster had loadbalancers, and the new one does not, the old scrape config will be removed.
		tasks = append(tasks, Task{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.RemoveLBScrapeConfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "removing loadbalancer scrabe config",
		})
	}

	// Create a scrape-config if there are loadbalancers in the new/updated cluster.
	if len(work.DesiredLoadbalancers) > 0 {
		tasks = append(tasks, Task{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreLBScrapeConfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "storing loadbalancer scrabe config",
		})
	}

	tasks = append(tasks, []Task{
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
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
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreKubeconfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "creating kubeconfig secret",
		},
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreClusterMetadata(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "creating cluster metadata secret",
		},
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.PatchNodes(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "patching k8s nodes",
		},
	}...)

	if work.DesiredCluster.AnyAutoscaledNodePools() {
		// Set up Autoscaler if desired state is autoscaled.
		tasks = append(tasks, Task{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.SetUpClusterAutoscaler(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deploying cluster autoscaler",
		})
	} else if work.CurrentCluster.AnyAutoscaledNodePools() {
		// Destroy Autoscaler if current state is autoscaled, but desired is not.
		tasks = append(tasks, Task{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DestroyClusterAutoscaler(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting cluster autoscaler",
		})
	}

	for _, t := range tasks {
		if err := u.tryProcessTask(ctx, work, logger, t); err != nil {
			return fmt.Errorf("failed to process task: %s: %w", work.Workflow.Description, err)
		}
	}

	return nil
}

// deleteClusterData deletes the kubeconfig, cluster metadata and cluster autoscaler from management cluster.
func (u *Usecases) deleteClusterData(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	if work.CurrentCluster == nil {
		return nil
	}

	tasks := []Task{
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DeleteKubeconfig(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting kubeconfig secret",
		},
		{
			do: func(_ context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DeleteClusterMetadata(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting cluster metadata secret",
		},
	}

	if work.CurrentCluster.AnyAutoscaledNodePools() {
		tasks = append(tasks, Task{
			do: func(ctx context.Context, bx *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.DestroyClusterAutoscaler(work, u.Kuber.GetClient())
			},
			stage:       spec.Workflow_KUBER,
			description: "deleting cluster autoscaler",
		})
	}

	for _, t := range tasks {
		if err := u.tryProcessTask(ctx, work, logger, t); err != nil {
			return fmt.Errorf("failed to process task: %s: %w", work.Workflow.Description, err)
		}
	}

	return nil
}

// callDeleteNodes calls Kuber.DeleteNodes which will gracefully delete nodes from cluster
func (u *Usecases) callDeleteNodes(cluster *spec.K8Scluster, nodepools map[string]*spec.DeletedNodes) (*spec.K8Scluster, error) {
	logger := loggerutils.WithClusterName(cluster.ClusterInfo.Id())

	logger.Info().Msg("Calling DeleteNodes on Kuber")
	resDelete, err := u.Kuber.DeleteNodes(cluster, nodepools, u.Kuber.GetClient())
	if err != nil {
		return nil, err
	}
	logger.Info().Msg("DeleteNodes on Kuber finished successfully")
	return resDelete.Cluster, nil
}
