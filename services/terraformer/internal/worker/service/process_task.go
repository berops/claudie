package service

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

type (
	Work struct {
		InputManifestName string
		Task              *spec.TaskV2

		// Passes are the individual transformations
		// that should be done with the task.
		Passes []*spec.StageTerraformer_SubPass
	}

	Stores struct {
		s3     store.S3StateStorage
		dynamo store.DynamoDB
	}

	Tracker struct {
		Result      *spec.TaskResult
		Diagnostics *Diagnostics
	}

	Diagnostics []string
)

func (d *Diagnostics) Push(val string) { (*d) = append(*d, val) }
func (d *Diagnostics) String() string  { return fmt.Sprint(*d) }

func ProcessTask(ctx context.Context, stores Stores, work Work) *spec.TaskResult {
	logger, ok := loggerutils.Value(ctx)
	if !ok {
		// this on should be set, but in case have a default one.
		logger = log.With().Logger()
		logger.Warn().Msg("No logger attached, using default")
	}

	processlimit, ok := processlimit.Value(ctx)
	if !ok {
		logger.Warn().Msg("No process limit found, using default")
		processlimit = semaphore.NewWeighted(5)
	}

	var (
		// diags holds any errors throughout all of the passes.
		diags Diagnostics

		// state the current state of the progress in [Work].
		// Start with None and update result as passes make changes.
		result = spec.TaskResult{Result: &spec.TaskResult_None_{None: new(spec.TaskResult_None)}}
	)

passes:
	for _, pass := range work.Passes {
		logger := logger.With().Str("terraform-stage", pass.Kind.String()).Logger()

		select {
		case <-ctx.Done():
			err := ctx.Err()
			logger.Err(err).Msg("Stopped passing state through passes, context cancelled")
			diags.Push(err.Error())
			break passes
		default:
		}

		tracker := Tracker{
			Result:      &result,
			Diagnostics: &diags,
		}
		last := len(diags)

		switch pass.Kind {
		case spec.StageTerraformer_BUILD_INFRASTRUCTURE:
			logger.Info().Msg("Bulding infrastructure")
			build(logger, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageTerraformer_DESTROY_INFRASTRUCTURE:
			logger.Info().Msg("Destroying infrastructure")
			destroy(logger, stores, work.InputManifestName, processlimit, work.Task, tracker)
		default:
			logger.Warn().Msg("Stage not recognized, skipping")
			continue
		}

		if current := len(diags); current > last {
			switch pass.Description.ErrorLevel {
			case spec.ErrorLevel_ERROR_FATAL:
				diags := diags[last:]

				logger.
					Err(fmt.Errorf("%v", diags)).
					Msg("Task failed for the current subpass, stopped processing task, propagating error")

				break passes
			case spec.ErrorLevel_ERROR_WARN:
				logger.
					Warn().
					Msg("Task failed for the current subpass, ignoring error and continuing with next")

				continue passes
			}
		}
	}

	if len(diags) > 0 {
		result.Error = &spec.TaskResult_Error{
			Kind:        spec.TaskResult_Error_PARTIAL,
			Description: fmt.Sprint(diags),
		}
	}

	return &result
}

func build(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	var (
		current, desired     *spec.K8SclusterV2
		currentLB, desiredLB []*spec.LBclusterV2
	)

	switch action := task.GetDo().(type) {
	case *spec.TaskV2_Create:
		current, currentLB = nil, nil
		desired = action.Create.GetK8S()
		if desired == nil {
			logger.Warn().Msg("create task validation failed, required desired state of the kuberentes cluster to be presetn, ignoring")
			tracker.Result.KeepAsIs()
			return
		}
		desiredLB = action.Create.GetLoadBalancers()
	case *spec.TaskV2_Update:
		update, ok := action.Update.GetOp().(*spec.UpdateV2_State_)
		if !ok {
			logger.Warn().Msgf("unknown update action to perform %T, ignoring", action.Update.GetOp())
			tracker.Result.KeepAsIs()
			return
		}
		current, desired = update.State.GetK8S().GetCurrent(), update.State.GetK8S().GetDesired()
		if current == nil || desired == nil {
			logger.
				Warn().
				Msg("update task validation failed, required kuberentes current,desired state to be present, but one of them is missing, ignoring")
			tracker.Result.KeepAsIs()
			return
		}

		for _, lb := range update.State.GetLoadBalancers() {
			if lb.Current == nil || lb.Desired == nil {
				logger.
					Warn().
					Msg("update task validation failed, required loadbalancer current,desired state to be present, but one of them is missing, ignoring")
				tracker.Result.KeepAsIs()
				return
			}
			currentLB = append(currentLB, lb.Current)
			desiredLB = append(desiredLB, lb.Desired)
		}
	case *spec.TaskV2_Delete:
		logger.Warn().Msgf("received delete task while wanting to build infrastructure, assuming the task was misscheduled, ignoring")
		tracker.Result.KeepAsIs()
		return
	default:
		logger.Warn().Msgf("unknown action to perform %T, ignoring", action)
		tracker.Result.KeepAsIs()
		return
	}

	cluster := kubernetes.K8Scluster{
		ProjectName:       projectName,
		DesiredState:      desired,
		CurrentState:      current,
		ExportPort6443:    clusters.FindAssignedLbApiEndpointV2(desiredLB) == nil,
		SpawnProcessLimit: processLimit,
	}

	if spec.OptionIsSet(task.Options, spec.ForceExportPort6443OnControlPlane) {
		cluster.ExportPort6443 = true
	}

	buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()

	if err := BuildK8Scluster(buildLogger, cluster); err != nil {
		// If the building of the cluster failed, we want the result to be a NOOP
		// as we do not want to update the current state in any way, as on failure
		// the just called function also cleans up after failing to run OpenTofu
		// thus the infra shouldn't have changed, though there are risks that OpenTofu
		// could fail (due to network conditions) but that up to the caller to handle
		// the error, we just propagate it from here.
		tracker.Diagnostics.Push(err.Error())
		tracker.Result.KeepAsIs()
		return
	}

	buildLogger.Info().Msg("Infrastructure for kubernetes cluster build successfully")

	if spec.OptionIsSet(task.Options, spec.K8sOnlyRefresh) {
		updatedCluster := cluster.CurrentState

		// Processing an event that only targets the nodepools used within the k8s
		// clusters, thus we do not need to update/refresh the loadbalancer and dns
		// infrastructure here. This is only done here for the purpose of shaving off
		// a few minutes from the build process.
		tracker.Result.ToUpdate().TakeKubernetesCluster(updatedCluster).Replace()
		return
	}

	var loadbalancers []loadbalancer.LBcluster
	for _, desired := range desiredLB {
		var match *spec.LBclusterV2

		for _, current := range currentLB {
			if desired.ClusterInfo.Id() == current.ClusterInfo.Id() {
				match = current
				break
			}
		}

		loadbalancers = append(loadbalancers, loadbalancer.LBcluster{
			ProjectName:       projectName,
			DesiredState:      desired,
			CurrentState:      match,
			SpawnProcessLimit: processLimit,
		})
	}

	err := concurrent.Exec(loadbalancers, func(_ int, cluster loadbalancer.LBcluster) error {
		buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
		return BuildLoadbalancers(buildLogger, cluster)
	})
	if err != nil {
		// Some part of loadbalancer infrastructure was not build successfully.
		// Since we still want to report the partially build infrastructure back to the
		// caller we fallthrough here, as any of the lbs successfully build infrastructure
		// will have its [CurrentState] updated.
		tracker.Diagnostics.Push(err.Error())
	}

	var (
		updatedK8s           = cluster.CurrentState
		updatedLoadBalancers []*spec.LBclusterV2
	)

	for _, lb := range loadbalancers {
		updatedLoadBalancers = append(updatedLoadBalancers, lb.CurrentState)
	}

	tracker.
		Result.
		ToUpdate().
		TakeKubernetesCluster(updatedK8s).
		TakeLoadBalancers(updatedLoadBalancers...).
		Replace()
}

func destroy(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	var clusters []Cluster

	action, ok := task.GetDo().(*spec.TaskV2_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to destroy infrastructure, assuming the task was misscheduled, ignoring", task.GetDo())

		tracker.Result.KeepAsIs()
		return
	}

	switch do := action.Delete.GetOp().(type) {
	case *spec.DeleteV2_Clusters_:
		k8s := do.Clusters.GetK8S()
		loadbalancers := do.Clusters.GetLoadBalancers()

		if k8s == nil {
			logger.
				Warn().
				Msg("delete task validation failed, required kubernetes state to be present, but is missing, ignoring")
			tracker.Result.KeepAsIs()
			return
		}

		clusters = append(clusters, &kubernetes.K8Scluster{
			ProjectName:       projectName,
			CurrentState:      k8s,
			SpawnProcessLimit: processLimit,
		})

		for _, lb := range loadbalancers {
			if lb == nil {
				logger.
					Warn().
					Msg("delete task validation failed, required loadbalancer state to be present, but is missing, ignoring")
				tracker.Result.KeepAsIs()
				return
			}

			clusters = append(clusters, &loadbalancer.LBcluster{
				ProjectName:       projectName,
				CurrentState:      lb,
				SpawnProcessLimit: processLimit,
			})
		}
	case *spec.DeleteV2_Loadbalancers:
		for _, lb := range do.Loadbalancers.GetLoadBalancers() {
			if lb == nil {
				logger.
					Warn().
					Msg("delete task validation failed, required loadbalancer state to be present, but is missing, ignoring")
				tracker.Result.KeepAsIs()
				return
			}

			clusters = append(clusters, &loadbalancer.LBcluster{
				ProjectName:       projectName,
				CurrentState:      lb,
				SpawnProcessLimit: processLimit,
			})
		}
	default:
		logger.Warn().Msgf("received unsupported delete action %T ignoring", action.Delete.GetOp())
		tracker.Result.KeepAsIs()
		return
	}

	ids := make([]string, len(clusters))
	err := concurrent.Exec(clusters, func(idx int, cluster Cluster) error {
		buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
		ids[idx] = cluster.Id()
		return DestroyCluster(buildLogger, projectName, cluster, stores.s3, stores.dynamo)
	})
	if err != nil {
		// Some of the provided clusters didn't destroy successfully.
		// Since we still want to report the partially destroyed infrastructure
		// back to the caller we fallthrough here, as any of the successfully destroyed
		// infrastructure will have its [CurrentState] updated to [nil].
		tracker.Diagnostics.Push(err.Error())
	}

	var (
		k8s string
		lbs []string
	)

	for i, c := range clusters {
		if !c.HasCurrentState() {
			if c.IsKubernetes() {
				k8s = ids[i]
			} else {
				lbs = append(lbs, ids[i])
			}
		}
	}

	tracker.
		Result.
		ToClear().
		TakeKuberentesCluster(k8s != "").
		TakeLoadBalancers(lbs...).
		Replace()
}
