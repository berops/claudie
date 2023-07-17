package usecases

import (
	"errors"
	"fmt"
	"strings"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
)

const (
	// maxDeleteRetry defines how many times the config should try to be deleted before returning an error, if encountered.
	maxDeleteRetry = 3
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

// saveWorkflowDescription sets description for a given builder context and saves it to Claudie database.
func (u *Usecases) saveWorkflowDescription(ctx *utils.BuilderContext, description string, cboxClient pb.ContextBoxServiceClient) error {
	ctx.Workflow.Description = strings.TrimSpace(description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	return nil
}

// deleteConfig calls destroy config to remove all traces of infrastructure from given config.
func (u *Usecases) deleteConfig(config *pb.Config, clusterView *cutils.ClusterView, cboxClient pb.ContextBoxServiceClient) error {
	var err error
	// Try maxDeleteRetry to delete the config.
	for i := 0; i < maxDeleteRetry; i++ {
		if err = u.destroyConfig(config, clusterView, cboxClient); err == nil {
			// Deletion successful, break here.
			break
		}
	}
	return err
}

// deleteCluster calls destroy cluster to remove all traces of infrastructure from cluster.
func (u *Usecases) deleteCluster(configName, clusterName string, clusterView *cutils.ClusterView, cboxClient pb.ContextBoxServiceClient) error {
	deleteCtx := &utils.BuilderContext{
		ProjectName:          configName,
		CurrentCluster:       clusterView.CurrentClusters[clusterName],
		CurrentLoadbalancers: clusterView.DeletedLoadbalancers[clusterName],
		Workflow:             clusterView.ClusterWorkflows[clusterName],
	}

	if err := u.destroyCluster(deleteCtx, cboxClient); err != nil {
		return err
	}
	return nil
}

// deleteNodes deletes nodes from the cluster based on the node map specified.
func (u *Usecases) deleteNodes(currentCluster, desiredCluster *pb.K8Scluster, nodes map[string]int32) (*pb.K8Scluster, error) {
	master, worker := utils.SeparateNodepools(nodes, currentCluster.ClusterInfo, desiredCluster.ClusterInfo)
	newCluster, err := u.callDeleteNodes(master, worker, currentCluster)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes for %s : %w", currentCluster.ClusterInfo.Name, err)
	}

	return newCluster, nil
}

// applyIR applies intermediate representation of the infrastructure to the Claudie workflow.
func (u *Usecases) applyIR(configName, clusterName string, clusterView *cutils.ClusterView, cboxClient pb.ContextBoxServiceClient, diff *utils.IntermediateRepresentation) (*utils.BuilderContext, error) {
	ctx := &utils.BuilderContext{
		ProjectName:    configName,
		CurrentCluster: clusterView.CurrentClusters[clusterName],
		DesiredCluster: diff.IR,

		// If there are any Lbs for the current state keep them.
		// Ignore the desired state for the Lbs for now. Use the
		// current state for desired to not trigger any changes.
		// as we only care about addition of nodes in this step.
		CurrentLoadbalancers: clusterView.Loadbalancers[clusterName],
		DesiredLoadbalancers: clusterView.Loadbalancers[clusterName],
		DeletedLoadBalancers: nil,

		Workflow: clusterView.ClusterWorkflows[clusterName],
	}

	if ctx, err := u.buildCluster(ctx, cboxClient); err != nil {
		clusterView.CurrentClusters[clusterName] = ctx.DesiredCluster
		clusterView.Loadbalancers[clusterName] = ctx.DesiredLoadbalancers

		if errors.Is(err, ErrFailedToBuildInfrastructure) {
			clusterView.CurrentClusters[clusterName] = ctx.CurrentCluster
			clusterView.Loadbalancers[clusterName] = ctx.CurrentLoadbalancers
		}
		return ctx, err
	}
	return ctx, nil
}

// applyAPIEndpointReplacement applies workflow for kube API Endpoint replacement.
func (u *Usecases) applyAPIEndpointReplacement(configName, clusterName string, clusterView *cutils.ClusterView, cboxClient pb.ContextBoxServiceClient) error {
	ctx := &utils.BuilderContext{
		ProjectName:    configName,
		CurrentCluster: clusterView.CurrentClusters[clusterName],
		DesiredCluster: clusterView.DesiredClusters[clusterName],
		Workflow:       clusterView.ClusterWorkflows[clusterName],
	}

	if err := u.callUpdateAPIEndpoint(ctx, cboxClient); err != nil {
		return err
	}

	clusterView.CurrentClusters[clusterName] = ctx.CurrentCluster
	clusterView.DesiredClusters[clusterName] = ctx.DesiredCluster

	ctx = &utils.BuilderContext{
		ProjectName:          configName,
		DesiredCluster:       clusterView.CurrentClusters[clusterName],
		DesiredLoadbalancers: clusterView.Loadbalancers[clusterName],
		Workflow:             clusterView.ClusterWorkflows[clusterName],
	}

	// Reconcile k8s cluster to assure new API endpoint has correct certificates.
	if err := u.reconcileK8sCluster(ctx, cboxClient); err != nil {
		return err
	}

	clusterView.CurrentClusters[clusterName] = ctx.DesiredCluster
	clusterView.Loadbalancers[clusterName] = ctx.DesiredLoadbalancers

	// Patch cluster-info config map to update certificates.
	if err := u.callPatchClusterInfoConfigMap(ctx, cboxClient); err != nil {
		return err
	}
	return nil
}
