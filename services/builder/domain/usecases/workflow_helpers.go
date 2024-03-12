package usecases

import (
	"errors"
	"fmt"
	"github.com/berops/claudie/services/builder/domain/usecases/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
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
	// LB add nodes prometheus metrics.
	for _, lb := range ctx.DesiredLoadbalancers {
		var currNodes int
		if idx := cutils.GetLBClusterByName(lb.ClusterInfo.Name, ctx.CurrentLoadbalancers); idx >= 0 {
			currNodes = cutils.CountLbNodes(ctx.CurrentLoadbalancers[idx])
		}

		adding := max(0, cutils.CountLbNodes(lb)-currNodes)

		metrics.LbAddingNodesInProgress.With(prometheus.Labels{
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(float64(adding))

		defer func(k8s, lb string, c int) {
			metrics.LbAddingNodesInProgress.With(prometheus.Labels{
				metrics.LBClusterLabel:     lb,
				metrics.K8sClusterLabel:    k8s,
				metrics.InputManifestLabel: ctx.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, adding)

		deleting := -min(cutils.CountLbNodes(lb)-currNodes, 0)

		metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(float64(deleting))

		defer func(k8s, lb string, c int) {
			metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
				metrics.K8sClusterLabel:    k8s,
				metrics.LBClusterLabel:     lb,
				metrics.InputManifestLabel: ctx.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, deleting)
	}

	metrics.K8sAddingNodesInProgress.With(prometheus.Labels{
		metrics.K8sClusterLabel:    ctx.GetClusterName(),
		metrics.InputManifestLabel: ctx.ProjectName,
	}).Add(float64(
		max(0, cutils.CountNodes(ctx.DesiredCluster)-cutils.CountNodes(ctx.CurrentCluster)),
	))

	defer func(c int) {
		metrics.K8sAddingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    ctx.GetClusterName(),
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(-float64(c))
	}(max(0, cutils.CountNodes(ctx.DesiredCluster)-cutils.CountNodes(ctx.CurrentCluster)))

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
	// K8s delete nodes prometheus metric.
	metrics.K8sDeletingNodesInProgress.With(prometheus.Labels{
		metrics.K8sClusterLabel:    ctx.GetClusterName(),
		metrics.InputManifestLabel: ctx.ProjectName,
	}).Add(float64(cutils.CountNodes(ctx.CurrentCluster)))

	defer func(c int) {
		metrics.K8sDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    ctx.GetClusterName(),
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(-float64(c))
	}(cutils.CountNodes(ctx.CurrentCluster))

	// LB delete nodes prometheus metrics.
	for _, lb := range ctx.CurrentLoadbalancers {
		metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(float64(cutils.CountLbNodes(lb)))

		defer func(k8s, lb string, c int) {
			metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
				metrics.K8sClusterLabel:    k8s,
				metrics.LBClusterLabel:     lb,
				metrics.InputManifestLabel: ctx.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, cutils.CountLbNodes(lb))
	}

	metrics.LBClustersInDeletion.Add(float64(len(ctx.CurrentLoadbalancers)))
	defer func(c int) { metrics.LBClustersInDeletion.Add(-float64(c)) }(len(ctx.CurrentLoadbalancers))

	if s := cutils.GetCommonStaticNodePools(ctx.CurrentCluster.GetClusterInfo().GetNodePools()); len(s) > 0 {
		if err := u.destroyK8sCluster(ctx, cboxClient); err != nil {
			log.Error().Msgf("error in destroy Kube-Eleven for config %s project %s : %v", ctx.GetClusterName(), ctx.ProjectName, err)
		}

		if err := u.removeClaudieUtilities(ctx, cboxClient); err != nil {
			log.Error().Msgf("error while removing claudie installed utilities for config %s project %s: %v", ctx.GetClusterName(), ctx.ProjectName, err)
		}
	}

	// Destroy infrastructure for the given cluster.
	if err := u.destroyInfrastructure(ctx, cboxClient); err != nil {
		return fmt.Errorf("error in destroy config Terraformer for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Delete Cluster data from management cluster.
	if err := u.deleteClusterData(ctx, cboxClient); err != nil {
		return fmt.Errorf("error in delete kubeconfig for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	metrics.LBClustersDeleted.Add(float64(len(ctx.CurrentLoadbalancers)))

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
func (u *Usecases) saveWorkflowDescription(ctx *utils.BuilderContext, description string, cboxClient pb.ContextBoxServiceClient) {
	log := cutils.CreateLoggerWithProjectName(ctx.ProjectName)
	ctx.Workflow.Description = strings.TrimSpace(description)
	if err := u.ContextBox.SaveWorkflowState(ctx.ProjectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		log.Err(err).Msgf("failed to update workflow description")
	}
}

// deleteConfig calls destroy config to remove all traces of infrastructure from given config.
func (u *Usecases) deleteConfig(config *pb.Config, clusterView *cutils.ClusterView, cboxClient pb.ContextBoxServiceClient) error {
	log := cutils.CreateLoggerWithProjectName(config.Name)

	var err error
	for i := 0; i < maxDeleteRetry; i++ {
		if err = u.destroyConfig(config, clusterView, cboxClient); err == nil {
			break
		}
		log.Err(err).Msg("failed to destroy config")
	}
	return err
}

// deleteCluster calls destroy cluster to remove all traces of infrastructure from cluster.
func (u *Usecases) deleteCluster(configName, clusterName string, clusterView *cutils.ClusterView, cboxClient pb.ContextBoxServiceClient) error {
	log := cutils.CreateLoggerWithProjectAndClusterName(configName, clusterName)

	deleteCtx := &utils.BuilderContext{
		ProjectName:          configName,
		CurrentCluster:       clusterView.CurrentClusters[clusterName],
		CurrentLoadbalancers: clusterView.DeletedLoadbalancers[clusterName],
		Workflow:             clusterView.ClusterWorkflows[clusterName],
	}

	var err error
	for i := 0; i < maxDeleteRetry; i++ {
		if err = u.destroyCluster(deleteCtx, cboxClient); err == nil {
			break
		}
		log.Err(err).Msg("failed to destroy cluster")
	}

	return err
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
		ProjectName:          configName,
		CurrentCluster:       clusterView.CurrentClusters[clusterName],
		DesiredCluster:       diff.IR,
		CurrentLoadbalancers: clusterView.Loadbalancers[clusterName],
		DesiredLoadbalancers: diff.IRLbs,
		DeletedLoadBalancers: nil,
		Workflow:             clusterView.ClusterWorkflows[clusterName],
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
