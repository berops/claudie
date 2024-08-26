package usecases

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/metrics"
	builder "github.com/berops/claudie/services/builder/internal"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/docker/distribution/context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

const (
	// maxDeleteRetry defines how many times the config should try to be deleted before returning an error, if encountered.
	maxDeleteRetry = 3
)

// buildCluster performs whole Claudie workflow on the given cluster.
func (u *Usecases) buildCluster(ctx *builder.Context) (*builder.Context, error) {
	// LB add nodes prometheus metrics.
	for _, lb := range ctx.DesiredLoadbalancers {
		var currNodes int
		if idx := utils.GetLBClusterByName(lb.ClusterInfo.Name, ctx.CurrentLoadbalancers); idx >= 0 {
			currNodes = utils.CountLbNodes(ctx.CurrentLoadbalancers[idx])
		}

		adding := max(0, utils.CountLbNodes(lb)-currNodes)

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

		deleting := -min(utils.CountLbNodes(lb)-currNodes, 0)

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
		max(0, utils.CountNodes(ctx.DesiredCluster)-utils.CountNodes(ctx.CurrentCluster)),
	))

	defer func(c int) {
		metrics.K8sAddingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    ctx.GetClusterName(),
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(-float64(c))
	}(max(0, utils.CountNodes(ctx.DesiredCluster)-utils.CountNodes(ctx.CurrentCluster)))

	// Reconcile infrastructure via terraformer.
	if err := u.reconcileInfrastructure(ctx); err != nil {
		return ctx, fmt.Errorf("error in Terraformer for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Configure infrastructure via Ansibler.
	if err := u.configureInfrastructure(ctx); err != nil {
		return ctx, fmt.Errorf("error in Ansibler for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Build k8s cluster via Kube-eleven.
	if err := u.reconcileK8sCluster(ctx); err != nil {
		return ctx, fmt.Errorf("error in Kube-eleven for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Reconcile k8s configuration via Kuber.
	if err := u.reconcileK8sConfiguration(ctx); err != nil {
		return ctx, fmt.Errorf("error in Kuber for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	return ctx, nil
}

// destroyCluster destroys existing clusters infrastructure for a config and cleans up management cluster from any of the cluster data.
func (u *Usecases) destroyCluster(ctx *builder.Context) error {
	// K8s delete nodes prometheus metric.
	metrics.K8sDeletingNodesInProgress.With(prometheus.Labels{
		metrics.K8sClusterLabel:    ctx.GetClusterName(),
		metrics.InputManifestLabel: ctx.ProjectName,
	}).Add(float64(utils.CountNodes(ctx.CurrentCluster)))

	defer func(c int) {
		metrics.K8sDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    ctx.GetClusterName(),
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(-float64(c))
	}(utils.CountNodes(ctx.CurrentCluster))

	// LB delete nodes prometheus metrics.
	for _, lb := range ctx.CurrentLoadbalancers {
		metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.InputManifestLabel: ctx.ProjectName,
		}).Add(float64(utils.CountLbNodes(lb)))

		defer func(k8s, lb string, c int) {
			metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
				metrics.K8sClusterLabel:    k8s,
				metrics.LBClusterLabel:     lb,
				metrics.InputManifestLabel: ctx.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, utils.CountLbNodes(lb))
	}

	metrics.LBClustersInDeletion.Add(float64(len(ctx.CurrentLoadbalancers)))
	defer func(c int) { metrics.LBClustersInDeletion.Add(-float64(c)) }(len(ctx.CurrentLoadbalancers))

	if s := utils.GetCommonStaticNodePools(ctx.CurrentCluster.GetClusterInfo().GetNodePools()); len(s) > 0 {
		if err := u.destroyK8sCluster(ctx); err != nil {
			log.Error().Msgf("error in destroy Kube-Eleven for config %s project %s : %v", ctx.GetClusterName(), ctx.ProjectName, err)
		}

		if err := u.removeClaudieUtilities(ctx); err != nil {
			log.Error().Msgf("error while removing claudie installed utilities for config %s project %s: %v", ctx.GetClusterName(), ctx.ProjectName, err)
		}
	}

	// Destroy infrastructure for the given cluster.
	if err := u.destroyInfrastructure(ctx); err != nil {
		return fmt.Errorf("error in destroy config Terraformer for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Delete Cluster data from management cluster.
	if err := u.deleteClusterData(ctx); err != nil {
		return fmt.Errorf("error in delete kubeconfig for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	metrics.LBClustersDeleted.Add(float64(len(ctx.CurrentLoadbalancers)))

	return nil
}

// deleteNodes deletes nodes from the cluster based on the node map specified.
func (u *Usecases) deleteNodes(current *spec.K8Scluster, nodepools map[string]*spec.DeletedNodes) (*spec.K8Scluster, error) {
	var master, worker []string
	for np, deleted := range nodepools {
		nodepool := utils.GetNodePoolByName(np, current.ClusterInfo.NodePools)
		if nodepool.IsControl {
			master = append(master, deleted.Nodes...)
		} else {
			worker = append(worker, deleted.Nodes...)
		}
	}

	newCluster, err := u.callDeleteNodes(master, worker, current)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes for %s : %w", current.ClusterInfo.Name, err)
	}

	return newCluster, nil
}

func (u *Usecases) updateTaskWithDescription(ctx *builder.Context, stage spec.Workflow_Stage, description string) {
	logger := utils.CreateLoggerWithProjectName(ctx.ProjectName)
	ctx.Workflow.Stage = stage
	ctx.Workflow.Description = strings.TrimSpace(description)

	err := u.Manager.TaskUpdate(context.Background(), &managerclient.TaskUpdateRequest{
		Config:  ctx.ProjectName,
		Cluster: ctx.GetClusterName(),
		TaskId:  ctx.TaskId,
		State:   ctx.Workflow,
	})
	if err != nil {
		logger.Debug().Msgf("failed to update state for task %q cluster %q config %q: %v", ctx.TaskId, ctx.GetClusterName(), ctx.ProjectName, err)
	}
}
