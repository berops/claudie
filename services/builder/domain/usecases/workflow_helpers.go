package usecases

import (
	"fmt"
	"strings"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

// buildCluster performs whole Claudie workflow on the given cluster.
func (u *Usecases) buildCluster(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) (*utils.BuilderContext, error) {
	// Reconcile infrastructure via terraformer.
	if err := u.reconcileInfrastructure(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Terraformer for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Configure infrastructure via Ansibler.
	if err := u.configureInfrastructure(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Ansibler for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Build k8s cluster via Kube-eleven.
	if err := u.reconcileK8sCluster(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Kube-eleven for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Reconcile k8s configuration via Kuber.
	if err := u.reconcileK8sConfiguration(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Kuber for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	return ctx, nil
}

// destroyCluster destroys existing clusters infrastructure for a config and cleans up management cluster from any of the cluster data.
func (u *Usecases) destroyCluster(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	// Destroy infrastructure for the given cluster.
	if err := u.destroyInfrastructure(ctx, cboxClient); err != nil {
		return fmt.Errorf("error in destroy config Terraformer for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Delete Cluster data from management cluster.
	if err := u.deleteClusterData(ctx, cboxClient); err != nil {
		return fmt.Errorf("error in delete kubeconfig for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	return nil
}

// destroyConfig destroys all the current state of the config.
func (u *Usecases) destroyConfig(config *pb.Config, clusterView *cutils.ClusterView, c pb.ContextBoxServiceClient) error {
	// Destroy all cluster concurrently.
	if err := cutils.ConcurrentExec(config.CurrentState.Clusters, func(_ int, cluster *pb.K8Scluster) error {
		err := u.destroyCluster(&utils.BuilderContext{
			ProjectName:          config.Name,
			CurrentCluster:       cluster,
			CurrentLoadbalancers: clusterView.Loadbalancers[cluster.ClusterInfo.Name],
			Workflow:             clusterView.ClusterWorkflows[cluster.ClusterInfo.Name],
		}, c)

		if err != nil {
			clusterView.SetWorkflowError(cluster.ClusterInfo.Name, err)
			return err
		}

		clusterView.SetWorkflowDone(cluster.ClusterInfo.Name)
		return nil
	}); err != nil {
		return err
	}

	return u.ContextBox.DeleteConfig(config, c)
}

func (u *Usecases) saveWorkflowDescription(ctx *utils.BuilderContext, description string, cboxClient pb.ContextBoxServiceClient) error {
	ctx.Workflow.Description = strings.TrimSpace(description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	return nil
}
