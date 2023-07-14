package usecases

import (
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	"github.com/rs/zerolog/log"
)

// reconcileK8sConfiguration reconciles desired k8s cluster configuration via kuber.
func (u *Usecases) reconcileK8sConfiguration(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())
	kuberClient := u.Kuber.GetClient()

	// Set workflow state.
	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_KUBER

	// Only patch cluster-info ConfigMap if kubeconfig changed.
	if ctx.CurrentCluster != nil && (ctx.CurrentCluster.Kubeconfig != ctx.DesiredCluster.Kubeconfig) {
		// Set new description.
		if err := u.callPatchClusterInfoConfigMap(ctx, cboxClient); err != nil {
			return err
		}
	}

	// If previous cluster had loadbalancers, and the new one does not, the old scrape config will be removed.
	if len(ctx.DesiredLoadbalancers) == 0 && len(ctx.CurrentLoadbalancers) > 0 {
		if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s removing Loadbalancer scrape config", description), cboxClient); err != nil {
			return err
		}
		logger.Info().Msg("Calling RemoveScrapeConfig on Kuber")
		if err := u.Kuber.RemoveLBScrapeConfig(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("RemoveScrapeConfig on Kuber finished successfully")
	}

	// Create a scrape-config if there are loadbalancers in the new/updated cluster.
	if len(ctx.DesiredLoadbalancers) > 0 {
		if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s storing Loadbalancer scrape config", description), cboxClient); err != nil {
			return err
		}
		logger.Info().Msg("Calling StoreLBScrapeConfig on Kuber")
		if err := u.Kuber.StoreLBScrapeConfig(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("StoreLBScrapeConfig on Kuber finished successfully")
	}

	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s setting up storage", description), cboxClient); err != nil {
		return err
	}

	log.Info().Msg("Calling SetUpStorage on Kuber for cluster")
	resStorage, err := u.Kuber.SetUpStorage(ctx, kuberClient)
	if err != nil {
		return err
	}
	logger.Info().Msg("SetUpStorage on Kuber finished successfully")

	ctx.DesiredCluster = resStorage.DesiredCluster
	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s creating kubeconfig secret", description), cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling StoreKubeconfig on kuber")
	if err := u.Kuber.StoreKubeconfig(ctx, kuberClient); err != nil {
		return err
	}
	logger.Info().Msg("StoreKubeconfig on Kuber finished successfully")

	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s creating cluster metadata secret", description), cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling StoreNodeMetadata on kuber")
	if err := u.Kuber.StoreClusterMetadata(ctx, kuberClient); err != nil {
		return err
	}
	logger.Info().Msg("StoreNodeMetadata on Kuber finished successfully")

	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s patching k8s nodes", description), cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling PatchNodes on kuber")
	if err := u.Kuber.PatchNodes(ctx, kuberClient); err != nil {
		return err
	}

	if cutils.IsAutoscaled(ctx.DesiredCluster) {
		// Set up Autoscaler if desired state is autoscaled.
		if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s deploying Cluster Autoscaler", description), cboxClient); err != nil {
			return err
		}
		logger.Info().Msg("Calling SetUpClusterAutoscaler on kuber")
		if err := u.Kuber.SetUpClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	} else if cutils.IsAutoscaled(ctx.CurrentCluster) {
		// Destroy Autoscaler if current state is autoscaled, but desired is not.
		if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s deleting Cluster Autoscaler", description), cboxClient); err != nil {
			return err
		}
		logger.Info().Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := u.Kuber.DestroyClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	}

	if err := u.saveWorkflowDescription(ctx, description, cboxClient); err != nil {
		return err
	}

	return nil
}

// callPatchClusterInfoConfigMap patches cluster-info ConfigMap via kuber.
func (u *Usecases) callPatchClusterInfoConfigMap(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_KUBER

	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s patching cluster info config map", description), cboxClient); err != nil {
		return err
	}
	logger.Info().Msg("Calling PatchClusterInfoConfigMap on kuber for cluster")
	if err := u.Kuber.PatchClusterInfoConfigMap(ctx, u.Kuber.GetClient()); err != nil {
		return err
	}
	logger.Info().Msg("PatchClusterInfoConfigMap on Kuber for cluster finished successfully")

	return u.saveWorkflowDescription(ctx, description, cboxClient)
}

// deleteClusterData deletes the kubeconfig, cluster metadata and cluster autoscaler from management cluster.
func (u *Usecases) deleteClusterData(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	if ctx.CurrentCluster == nil {
		return nil
	}
	description := ctx.Workflow.Description
	kuberClient := u.Kuber.GetClient()
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	ctx.Workflow.Stage = pb.Workflow_DESTROY_KUBER
	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s deleting kubeconfig secret", description), cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling DeleteKubeconfig on Kuber")
	if err := u.Kuber.DeleteKubeconfig(ctx, kuberClient); err != nil {
		return fmt.Errorf("error while deleting kubeconfig for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s deleting cluster metadata secret", description), cboxClient); err != nil {
		return err
	}
	logger.Info().Msg("Calling DeleteClusterMetadata on kuber")
	if err := u.Kuber.DeleteClusterMetadata(ctx, kuberClient); err != nil {
		return fmt.Errorf("error while deleting metadata for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}
	logger.Info().Msg("DeleteKubeconfig on Kuber finished successfully")

	// Destroy Autoscaler if current state is autoscaled
	if cutils.IsAutoscaled(ctx.CurrentCluster) {
		if err := u.saveWorkflowDescription(ctx, fmt.Sprintf("%s deleting Cluster Autoscaler", description), cboxClient); err != nil {
			return err
		}
		logger.Info().Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := u.Kuber.DestroyClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	}

	if err := u.saveWorkflowDescription(ctx, description, cboxClient); err != nil {
		return err
	}

	return nil
}

// callDeleteNodes calls Kuber.DeleteNodes which will gracefully delete nodes from cluster
func (u *Usecases) callDeleteNodes(master, worker []string, cluster *pb.K8Scluster) (*pb.K8Scluster, error) {
	logger := cutils.CreateLoggerWithClusterName(cutils.GetClusterID(cluster.ClusterInfo))

	logger.Info().Msg("Calling DeleteNodes on Kuber")
	resDelete, err := u.Kuber.DeleteNodes(cluster, master, worker, u.Kuber.GetClient())
	if err != nil {
		return nil, err
	}
	logger.Info().Msg("DeleteNodes on Kuber finished successfully")
	return resDelete.Cluster, nil
}
