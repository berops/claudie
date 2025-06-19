package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/metrics"
	builder "github.com/berops/claudie/services/builder/internal"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Builds and configures the kuberentes and loadbalancer clusters as specified in the passed in builder.Context.
// If an error occurs during the processing, any partial changes will be stored in the passed in builder.Context.
func (u *Usecases) buildCluster(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	// LB add nodes prometheus metrics.
	for _, lb := range work.DesiredLoadbalancers {
		var currNodes int
		if idx := clusters.IndexLoadbalancerById(lb.ClusterInfo.Id(), work.CurrentLoadbalancers); idx >= 0 {
			currNodes = work.CurrentLoadbalancers[idx].NodeCount()
		}

		adding := max(0, lb.NodeCount()-currNodes)

		metrics.LbAddingNodesInProgress.With(prometheus.Labels{
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.InputManifestLabel: work.ProjectName,
		}).Add(float64(adding))

		defer func(k8s, lb string, c int) {
			metrics.LbAddingNodesInProgress.With(prometheus.Labels{
				metrics.LBClusterLabel:     lb,
				metrics.K8sClusterLabel:    k8s,
				metrics.InputManifestLabel: work.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, adding)

		deleting := -min(lb.NodeCount()-currNodes, 0)

		metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.InputManifestLabel: work.ProjectName,
		}).Add(float64(deleting))

		defer func(k8s, lb string, c int) {
			metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
				metrics.K8sClusterLabel:    k8s,
				metrics.LBClusterLabel:     lb,
				metrics.InputManifestLabel: work.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, deleting)
	}

	metrics.K8sAddingNodesInProgress.With(prometheus.Labels{
		metrics.K8sClusterLabel:    work.GetClusterName(),
		metrics.InputManifestLabel: work.ProjectName,
	}).Add(float64(
		max(0, work.DesiredCluster.NodeCount()-work.CurrentCluster.NodeCount()),
	))

	defer func(c int) {
		metrics.K8sAddingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    work.GetClusterName(),
			metrics.InputManifestLabel: work.ProjectName,
		}).Add(-float64(c))
	}(max(0, work.DesiredCluster.NodeCount()-work.CurrentCluster.NodeCount()))

	tasks := []Task{
		{
			do:          u.reconcileInfrastructure,
			stage:       spec.Workflow_TERRAFORMER,
			description: "reconciling infrastructure",
		},
		{
			do:          u.configureInfrastructure,
			stage:       spec.Workflow_ANSIBLER,
			description: "configuring infrastructure",
		},
		{
			do:          u.reconcileK8sCluster,
			stage:       spec.Workflow_KUBE_ELEVEN,
			description: "reconciling cluster",
		},
		{
			do:          u.reconcileK8sConfiguration,
			stage:       spec.Workflow_KUBER,
			description: "reconciling cluster configuration",
		},
	}

	return u.processTasks(ctx, work, logger, tasks)
}

// destroyCluster destroys existing clusters infrastructure for a config and cleans up management cluster from any of the cluster data.
func (u *Usecases) destroyCluster(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
	// K8s delete nodes prometheus metric.
	metrics.K8sDeletingNodesInProgress.With(prometheus.Labels{
		metrics.K8sClusterLabel:    work.GetClusterName(),
		metrics.InputManifestLabel: work.ProjectName,
	}).Add(float64(work.CurrentCluster.NodeCount()))

	defer func(c int) {
		metrics.K8sDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    work.GetClusterName(),
			metrics.InputManifestLabel: work.ProjectName,
		}).Add(-float64(c))
	}(work.CurrentCluster.NodeCount())

	// LB delete nodes prometheus metrics.
	for _, lb := range work.CurrentLoadbalancers {
		metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
			metrics.K8sClusterLabel:    lb.TargetedK8S,
			metrics.LBClusterLabel:     lb.ClusterInfo.Name,
			metrics.InputManifestLabel: work.ProjectName,
		}).Add(float64(lb.NodeCount()))

		defer func(k8s, lb string, c int) {
			metrics.LbDeletingNodesInProgress.With(prometheus.Labels{
				metrics.K8sClusterLabel:    k8s,
				metrics.LBClusterLabel:     lb,
				metrics.InputManifestLabel: work.ProjectName,
			}).Add(-float64(c))
		}(lb.TargetedK8S, lb.ClusterInfo.Name, lb.NodeCount())
	}

	metrics.LBClustersInDeletion.Add(float64(len(work.CurrentLoadbalancers)))
	defer func(c int) { metrics.LBClustersInDeletion.Add(-float64(c)) }(len(work.CurrentLoadbalancers))

	var tasks []Task

	if s := nodepools.Static(work.CurrentCluster.GetClusterInfo().GetNodePools()); len(s) > 0 {
		tasks = append(tasks,
			Task{
				do:              u.destroyK8sCluster,
				stage:           spec.Workflow_KUBE_ELEVEN,
				description:     "destroying kuberentes cluster and uninstall binaries",
				continueOnError: true,
			},
			Task{
				do:              u.removeClaudieUtilities,
				stage:           spec.Workflow_ANSIBLER,
				description:     "removing claudie installed utilities",
				continueOnError: true,
			},
		)
	}

	tasks = append(tasks,
		Task{
			do:          u.destroyInfrastructure,
			stage:       spec.Workflow_TERRAFORMER,
			description: "destroying infrastructure",
		},
		Task{
			do:          u.deleteClusterData,
			stage:       spec.Workflow_KUBER,
			description: "cleanup cluster resources",
		},
	)

	if err := u.processTasks(ctx, work, logger, tasks); err != nil {
		return err
	}

	metrics.LBClustersDeleted.Add(float64(len(work.CurrentLoadbalancers)))
	return nil
}

func (u *Usecases) updateTaskWithDescription(ctx *builder.Context, stage spec.Workflow_Stage, description string) {
	logger := loggerutils.WithProjectName(ctx.ProjectName)
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

// Task processes parts or all of the infrastructure as specified in the passed in builder.Context.
// Any partial state change that the task does, should be reflected in the passed in builder.Context.
type Task struct {
	do              DoFn
	stage           spec.Workflow_Stage
	description     string
	continueOnError bool
	condition       func(work *builder.Context) bool
}

// Do function is what the above defined Task understands and will execute.
// This type may itself run tasks which may be scheduled for execution.
type DoFn func(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error

// Tries to execute the given task, if the context is cancelled the error form the context is returned
// and any partial changes done will be reflected in the passed in builder.Context, otherwise the passed
// in task is processed.
func (u *Usecases) tryProcessTask(
	ctx context.Context,
	work *builder.Context,
	logger *zerolog.Logger,
	t Task,
) error {
	description := work.Workflow.Description

	// skip updating the task description on errors, to keep the stage where the task failed.
	u.updateTaskWithDescription(work, t.stage, fmt.Sprintf("%s:%s", description, t.description))
	logger.Info().Msgf("Working on %q", work.Workflow.Description)

	select {
	case <-ctx.Done():
		err := ctx.Err()
		logger.Err(err).Msgf("No work done on %q", work.Workflow.Description)
		return err
	default:
		// continue
	}

	if t.condition != nil && !t.condition(work) {
		logger.Info().Msgf("Skip %q, as cluster does not meet condition", work.Workflow.Description)
		u.updateTaskWithDescription(work, t.stage, description)
		return nil
	}

	err := t.do(ctx, work, logger)
	if err != nil {
		if t.continueOnError {
			logger.Warn().Err(err).Msgf("Work: %q failed, ignoring", work.Workflow.Description)
			u.updateTaskWithDescription(work, t.stage, description)
			return nil
		}
		return err
	}

	logger.Info().Msgf("Work %q, finished successfully", work.Workflow.Description)
	u.updateTaskWithDescription(work, t.stage, description)
	return nil
}

func (u *Usecases) processTasks(ctx context.Context, work *builder.Context, logger *zerolog.Logger, tasks []Task) error {
	for _, t := range tasks {
		if err := u.tryProcessTask(ctx, work, logger, t); err != nil {
			return err
		}
	}
	return nil
}
