package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	builder "github.com/berops/claudie/services/builder/internal"
	"github.com/rs/zerolog/log"
)

// reconcileK8sConfiguration reconciles desired k8s cluster configuration via kuber.
func (u *Usecases) reconcileK8sConfiguration(ctx *builder.Context) error {
	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())
	kuberClient := u.Kuber.GetClient()

	// Set workflow state.
	description := ctx.Workflow.Description
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s processing kuber commands", description))

	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(ctx.DesiredLoadbalancers); ep != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	if err := u.Kuber.PatchKubeadmConfigMap(ctx, lbApiEndpoint, kuberClient); err != nil {
		return err
	}

	// Only patch ConfigMaps if kubeconfig changed.
	if ctx.CurrentCluster != nil && (ctx.CurrentCluster.Kubeconfig != ctx.DesiredCluster.Kubeconfig) {
		if err := u.callPatchClusterInfoConfigMap(ctx); err != nil {
			return err
		}
		if err := u.Kuber.PatchKubeProxyConfigMap(ctx, kuberClient); err != nil {
			return err
		}
	}

	if err := u.Kuber.CiliumRolloutRestart(ctx.DesiredCluster, kuberClient); err != nil {
		return err
	}

	// If previous cluster had loadbalancers, and the new one does not, the old scrape config will be removed.
	if len(ctx.DesiredLoadbalancers) == 0 && len(ctx.CurrentLoadbalancers) > 0 {
		u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s removing Loadbalancer scrape config", description))
		logger.Info().Msg("Calling RemoveScrapeConfig on Kuber")
		if err := u.Kuber.RemoveLBScrapeConfig(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("RemoveScrapeConfig on Kuber finished successfully")
	}

	// Create a scrape-config if there are loadbalancers in the new/updated cluster.
	if len(ctx.DesiredLoadbalancers) > 0 {
		u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s storing Loadbalancer scrape config", description))
		logger.Info().Msg("Calling StoreLBScrapeConfig on Kuber")
		if err := u.Kuber.StoreLBScrapeConfig(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("StoreLBScrapeConfig on Kuber finished successfully")
	}

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s setting up storage", description))

	log.Info().Msg("Calling SetUpStorage on Kuber for cluster")
	resStorage, err := u.Kuber.SetUpStorage(ctx, kuberClient)
	if err != nil {
		return err
	}
	logger.Info().Msg("SetUpStorage on Kuber finished successfully")

	ctx.DesiredCluster = resStorage.DesiredCluster
	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s creating kubeconfig secret", description))

	logger.Info().Msg("Calling StoreKubeconfig on kuber")
	if err := u.Kuber.StoreKubeconfig(ctx, kuberClient); err != nil {
		return err
	}
	logger.Info().Msg("StoreKubeconfig on Kuber finished successfully")

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s creating cluster metadata secret", description))

	logger.Info().Msg("Calling StoreNodeMetadata on kuber")
	if err := u.Kuber.StoreClusterMetadata(ctx, kuberClient); err != nil {
		return err
	}
	logger.Info().Msg("StoreNodeMetadata on Kuber finished successfully")

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s patching k8s nodes", description))

	logger.Info().Msg("Calling PatchNodes on kuber")
	if err := u.Kuber.PatchNodes(ctx, kuberClient); err != nil {
		return err
	}

	if ctx.DesiredCluster.AnyAutoscaledNodePools() {
		// Set up Autoscaler if desired state is autoscaled.
		u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s deploying Cluster Autoscaler", description))
		logger.Info().Msg("Calling SetUpClusterAutoscaler on kuber")
		if err := u.Kuber.SetUpClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	} else if ctx.CurrentCluster.AnyAutoscaledNodePools() {
		// Destroy Autoscaler if current state is autoscaled, but desired is not.
		u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s deleting Cluster Autoscaler", description))
		logger.Info().Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := u.Kuber.DestroyClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	}

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, description)
	return nil
}

// callPatchClusterInfoConfigMap patches cluster-info ConfigMap via kuber.
func (u *Usecases) callPatchClusterInfoConfigMap(ctx *builder.Context) error {
	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())

	description := ctx.Workflow.Description

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("%s patching cluster info config map", description))

	logger.Info().Msg("Calling PatchClusterInfoConfigMap on kuber for cluster")
	if err := u.Kuber.PatchClusterInfoConfigMap(ctx, u.Kuber.GetClient()); err != nil {
		return err
	}
	logger.Info().Msg("PatchClusterInfoConfigMap on Kuber for cluster finished successfully")

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, description)
	return nil
}

// deleteClusterData deletes the kubeconfig, cluster metadata and cluster autoscaler from management cluster.
func (u *Usecases) deleteClusterData(ctx *builder.Context) error {
	if ctx.CurrentCluster == nil {
		return nil
	}
	description := ctx.Workflow.Description
	kuberClient := u.Kuber.GetClient()
	logger := loggerutils.WithProjectAndCluster(ctx.ProjectName, ctx.Id())

	u.updateTaskWithDescription(ctx, spec.Workflow_DESTROY_KUBER, fmt.Sprintf("%s deleting kubeconfig secret", description))

	logger.Info().Msgf("Calling DeleteKubeconfig on Kuber")
	if err := u.Kuber.DeleteKubeconfig(ctx, kuberClient); err != nil {
		return fmt.Errorf("error while deleting kubeconfig for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	u.updateTaskWithDescription(ctx, spec.Workflow_DESTROY_KUBER, fmt.Sprintf("%s deleting cluster metadata secret", description))

	logger.Info().Msg("Calling DeleteClusterMetadata on kuber")
	if err := u.Kuber.DeleteClusterMetadata(ctx, kuberClient); err != nil {
		return fmt.Errorf("error while deleting metadata for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}
	logger.Info().Msg("DeleteKubeconfig on Kuber finished successfully")

	if ctx.CurrentCluster.AnyAutoscaledNodePools() {
		u.updateTaskWithDescription(ctx, spec.Workflow_DESTROY_KUBER, fmt.Sprintf("%s deleting Cluster Autoscaler", description))
		logger.Info().Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := u.Kuber.DestroyClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	}

	u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, description)
	return nil
}

// callDeleteNodes calls Kuber.DeleteNodes which will gracefully delete nodes from cluster
func (u *Usecases) callDeleteNodes(master, worker []string, cluster *spec.K8Scluster) (*spec.K8Scluster, error) {
	logger := loggerutils.WithClusterName(cluster.ClusterInfo.Id())

	logger.Info().Msg("Calling DeleteNodes on Kuber")
	resDelete, err := u.Kuber.DeleteNodes(cluster, master, worker, u.Kuber.GetClient())
	if err != nil {
		return nil, err
	}
	logger.Info().Msg("DeleteNodes on Kuber finished successfully")
	return resDelete.Cluster, nil
}
