package usecases

import (
	"fmt"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	"github.com/rs/zerolog/log"
)

func (u *Usecases) reconcileK8sConfiguration(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())
	kuberClient := u.Kuber.GetClient()

	description := ctx.Workflow.Description
	ctx.Workflow.Stage = pb.Workflow_KUBER

	// only patch if kubeconfig changed.
	if ctx.CurrentCluster != nil && (ctx.CurrentCluster.Kubeconfig != ctx.DesiredCluster.Kubeconfig) {
		ctx.Workflow.Description = fmt.Sprintf("%s patching cluster info config map", description)
		if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
			return err
		}

		logger.Info().Msg("Calling PatchClusterInfoConfigMap on kuber for cluster")
		if err := u.Kuber.PatchClusterInfoConfigMap(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("PatchClusterInfoConfigMap on Kuber for cluster finished successfully")
	}

	// If previous cluster had loadbalancers, and the new one does not, the old scrape config will be removed.
	if len(ctx.DesiredLoadbalancers) == 0 && len(ctx.CurrentLoadbalancers) > 0 {
		logger.Info().Msg("Calling RemoveScrapeConfig on Kuber")
		if err := u.Kuber.RemoveLBScrapeConfig(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("RemoveScrapeConfig on Kuber finished successfully")
	}

	// Create a scrape-config if there are loadbalancers in the new/updated cluster
	if len(ctx.DesiredLoadbalancers) > 0 {
		logger.Info().Msg("Calling StoreLBScrapeConfig on Kuber")
		if err := u.Kuber.StoreLBScrapeConfig(ctx, kuberClient); err != nil {
			return err
		}
		logger.Info().Msg("StoreLBScrapeConfig on Kuber finished successfully")
	}

	ctx.Workflow.Description = fmt.Sprintf("%s setting up storage", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msg("Calling SetUpStorage on Kuber for cluster")
	resStorage, err := u.Kuber.SetUpStorage(ctx, kuberClient)
	if err != nil {
		return err
	}
	logger.Info().Msg("SetUpStorage on Kuber finished successfully")

	ctx.DesiredCluster = resStorage.DesiredCluster

	ctx.Workflow.Description = fmt.Sprintf("%s creating kubeconfig as secret", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling StoreKubeconfig on kuber")
	if err := u.Kuber.StoreKubeconfig(ctx, kuberClient); err != nil {
		return err
	}
	logger.Info().Msg("StoreKubeconfig on Kuber finished successfully")

	ctx.Workflow.Description = fmt.Sprintf("%s creating cluster metadata as secret", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling StoreNodeMetadata on kuber")
	if err := u.Kuber.StoreClusterMetadata(ctx, kuberClient); err != nil {
		return err
	}
	logger.Info().Msg("StoreNodeMetadata on Kuber finished successfully")

	logger.Info().Msg("Calling PatchNodes on kuber")
	if err := u.Kuber.PatchNodes(ctx, kuberClient); err != nil {
		return err
	}

	if cutils.IsAutoscaled(ctx.DesiredCluster) {
		// Set up Autoscaler if desired state is autoscaled
		logger.Info().Msg("Calling SetUpClusterAutoscaler on kuber")
		if err := u.Kuber.SetUpClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	} else if cutils.IsAutoscaled(ctx.CurrentCluster) {
		// Destroy Autoscaler if current state is autoscaled, but desired is not
		logger.Info().Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := u.Kuber.DestroyClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	}

	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	return nil
}

// deleteClusterData deletes the kubeconfig and cluster metadata.
func (u *Usecases) deleteClusterData(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	if ctx.CurrentCluster == nil {
		return nil
	}
	description := ctx.Workflow.Description
	kuberClient := u.Kuber.GetClient()
	logger := cutils.CreateLoggerWithProjectAndClusterName(ctx.ProjectName, ctx.GetClusterID())

	ctx.Workflow.Stage = pb.Workflow_DESTROY_KUBER
	ctx.Workflow.Description = fmt.Sprintf("%s deleting kubeconfig secret", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msgf("Calling DeleteKubeconfig on Kuber")
	if err := u.Kuber.DeleteKubeconfig(ctx, kuberClient); err != nil {
		return fmt.Errorf("error while deleting kubeconfig for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	ctx.Workflow.Description = fmt.Sprintf("%s deleting cluster metadata secret", description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	logger.Info().Msg("Calling DeleteClusterMetadata on kuber")
	if err := u.Kuber.DeleteClusterMetadata(ctx, kuberClient); err != nil {
		return fmt.Errorf("error while deleting metadata for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}
	logger.Info().Msg("DeleteKubeconfig on Kuber finished successfully")
	ctx.Workflow.Description = description
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	// Destroy Autoscaler if current state is autoscaled
	if cutils.IsAutoscaled(ctx.CurrentCluster) {
		log.Info().Str("project", ctx.ProjectName).Str("cluster", ctx.GetClusterName()).Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := u.Kuber.DestroyClusterAutoscaler(ctx, kuberClient); err != nil {
			return err
		}
	}

	return nil
}

// deleteNodes calls Kuber.DeleteNodes which will safely delete nodes from cluster
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
