package usecases

import (
	"errors"
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

	// The proxy envs need to be created after terraformer phase.
	updateProxyEnvsFlag := false
	defaultMode := "default"
	offMode := "off"

	// If proxy mode isn't specified, set default value.
	if ctx.DesiredCluster.InstallationProxy.Mode == "" {
		ctx.DesiredCluster.InstallationProxy.Mode = defaultMode
	}

	if ctx.CurrentCluster != nil && ctx.DesiredCluster != nil {
		updateProxyEnvsFlag = true

		currProxySettings := ctx.CurrentCluster.InstallationProxy
		desiredProxySettings := ctx.DesiredCluster.InstallationProxy

		// proxy settings differs.
		// It was default without Hetzner node and it is turned off // NO CHANGE
		// It was default without Hetzner node and it is turned on // CHANGE
		// It was default with Hetzner node and it is turned off // CHANGE
		// It was default with Hetzner node and it is turned on // CHANGE
		// It was on and is turned off // CHANGE
		// It was on and is turned to default // CHANGE (It doesn't matter if the k8s cluster contains a Hetzner node or not)
		// It was off and is turned on // CHANGE
		// It was off and is turned to default with Hetzner node // CHANGE
		// It was off and is turned to default without Hetzner node // NO CHANGE

		// proxy settings are the same.
		// It is on // CHANGE
		// It is off // NO CHANGE
		// It is default with Hetzner node in desired state and current state // CHANGE
		// It is default with Hetzner node in desired and without Hetzner node in current state // CHANGE
		// It is default without Hetzner node in desired state and with Hetzner node in current state // CHANGE
		// It is default without Hetzner node in desired state and without Hetzner node in current state // NO CHANGE

		if currProxySettings.Mode == offMode && desiredProxySettings.Mode == offMode {
			// The proxy is and was turned off in both cases.
			updateProxyEnvsFlag = false
		} else if currProxySettings.Mode == defaultMode && desiredProxySettings.Mode == defaultMode &&
			!builder.HasHetznerNode(ctx.CurrentCluster.ClusterInfo) && !builder.HasHetznerNode(ctx.DesiredCluster.ClusterInfo) {
			// The proxy is in default mode without Hetzner node in both cases.
			updateProxyEnvsFlag = false
		} else if currProxySettings.Mode == defaultMode && !builder.HasHetznerNode(ctx.CurrentCluster.ClusterInfo) &&
			desiredProxySettings.Mode == offMode {
			// The proxy was in default mode without Hetzner node and is turned off.
			updateProxyEnvsFlag = false
		} else if currProxySettings.Mode == offMode && desiredProxySettings.Mode == defaultMode &&
			!builder.HasHetznerNode(ctx.DesiredCluster.ClusterInfo) {
			// The proxy was in off mode. Now it is in default mode without Hetzner node.
			updateProxyEnvsFlag = false
		}
	} else if ctx.DesiredCluster != nil {
		// The cluster wasn't build yet because currentState is nil
		// but we have to check if the proxy is turned on or in a default mode with Hetzner node in the desired state.
		desiredProxySettings := ctx.DesiredCluster.InstallationProxy

		if desiredProxySettings.Mode == defaultMode && builder.HasHetznerNode(ctx.DesiredCluster.ClusterInfo) {
			updateProxyEnvsFlag = true
		} else if desiredProxySettings.Mode == "on" {
			updateProxyEnvsFlag = true
		}
	}

	// HttProxyUrl and NoProxyList will be set before first task in ansibler and then updated after ansibler Install VPN phase.
	ctx.ProxyEnvs = &spec.ProxyEnvs{
		UpdateProxyEnvsFlag: updateProxyEnvsFlag,
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

func (u *Usecases) updateTaskWithDescription(ctx *builder.Context, stage spec.Workflow_Stage, description string) {
	logger := utils.CreateLoggerWithProjectName(ctx.ProjectName)
	ctx.Workflow.Stage = stage
	ctx.Workflow.Description = strings.TrimSpace(description)

	// ignore error, this is not a fatal error due to which we can't continue.
	_ = managerclient.Retry(&logger, "TaskUpdate", func() error {
		log.Debug().Msgf("updating task %q for cluster %q for config %q with state: %s", ctx.TaskId, ctx.GetClusterName(), ctx.ProjectName, ctx.Workflow.String())
		err := u.Manager.TaskUpdate(context.Background(), &managerclient.TaskUpdateRequest{
			Config:  ctx.ProjectName,
			Cluster: ctx.GetClusterName(),
			TaskId:  ctx.TaskId,
			State:   ctx.Workflow,
		})
		if errors.Is(err, managerclient.ErrNotFound) {
			log.Warn().Msgf("can't update config %q cluster %q task %q: %v", ctx.ProjectName, ctx.GetClusterName(), ctx.TaskId, err)
			return nil // nothing to retry
		}
		return err
	})
}
