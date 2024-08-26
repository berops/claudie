package usecases

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"
)

func (u *Usecases) TaskProcessor(wg *sync.WaitGroup) error {
	ctx := context.Background()

	task, err := u.Manager.NextTask(ctx)
	if err != nil || task == nil {
		if errors.Is(err, managerclient.ErrVersionMismatch) {
			log.Debug().Msgf("failed to receive next task due to a dirty write")
		}
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		updatedState, err := u.processTaskEvent(task)
		if err != nil {
			log.Err(err).Msgf("failed to process task %q for cluster %q for config %q", task.Event.Id, task.Cluster, task.Config)
			task.State.Status = spec.Workflow_ERROR
			task.State.Description = err.Error()
			// fallthrough
		} else {
			log.Info().Msgf("sucessfully processed task %q for cluster %q for config %q", task.Event.Id, task.Cluster, task.Config)
			task.State.Status = spec.Workflow_DONE
			task.State.Stage = spec.Workflow_NONE
			task.State.Description = ""
			// fallthrough
		}

		tolerateDirtyWrites := 5
		var errs error
		for i := range tolerateDirtyWrites {
			if i > 0 {
				wait := time.Duration(50+rand.IntN(300)) * time.Millisecond
				log.Warn().Msgf("retry[%v/%v]Failed to update current state due to a dirty write, retrying again in %s ms", i, tolerateDirtyWrites, wait)
				time.Sleep(wait)
			}

			err := u.Manager.UpdateCurrentState(ctx, &managerclient.UpdateCurrentStateRequest{
				Config:   task.Config,
				Cluster:  task.Cluster,
				Clusters: updatedState,
			})
			if err != nil {
				errs = errors.Join(errs, err)
				if errors.Is(err, managerclient.ErrVersionMismatch) {
					continue
				}
				break // unknown error, log.
			}

			err = u.Manager.TaskUpdate(ctx, &managerclient.TaskUpdateRequest{
				Config:  task.Config,
				Cluster: task.Cluster,
				TaskId:  task.Event.Id,
				State:   task.State,
			})
			if err != nil {
				errs = errors.Join(errs, err)
				if errors.Is(err, managerclient.ErrVersionMismatch) {
					continue
				}
				break // unknown error, log.
			}
			return // completed nothing to do.
		}
		if errs != nil {
			log.Err(err).Msgf("failed to update current state for cluster %q config %q", task.Cluster, task.Config)
		}
	}()
	return nil
}

func (u *Usecases) processTaskEvent(t *managerclient.NextTaskResponse) (*spec.Clusters, error) {
	var (
		err error
		k8s *spec.K8Scluster
		lbs []*spec.LBcluster
	)

	switch t.Event.Event {
	case spec.Event_CREATE:
		log.Debug().Msgf("[task %q] Create operation cluster %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeCreateTask(t)
	case spec.Event_UPDATE:
		log.Debug().Msgf("[task %q] Update operation %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeUpdateTask(t)
	case spec.Event_DELETE:
		log.Debug().Msgf("[task %q] Delete operation %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeDeleteTask(t)
	}

	var resp *spec.Clusters
	if k8s != nil {
		resp = &spec.Clusters{K8S: k8s}
		if len(lbs) != 0 {
			resp.LoadBalancers = &spec.LoadBalancers{Clusters: lbs}
		}
	}
	return resp, err
}

func (u *Usecases) executeCreateTask(te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	ctx := &utils.BuilderContext{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		DesiredCluster:       te.Event.Task.CreateState.K8S,
		DesiredLoadbalancers: te.Event.Task.CreateState.GetLbs().GetClusters(),
		Workflow:             te.State,
	}
	ctx, err := u.buildCluster(ctx)
	return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
}

func (u *Usecases) executeUpdateTask(te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	if te.Event.Task.UpdateState.ApiNodePool != "" {
		ctx := &utils.BuilderContext{
			ProjectName:    te.Config,
			TaskId:         te.Event.Id,
			CurrentCluster: te.Current.K8S,
			Workflow:       te.State,
		}

		if err := u.callUpdateAPIEndpoint(ctx, te.Event.Task.UpdateState.ApiNodePool); err != nil {
			return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
		}

		ctx = &utils.BuilderContext{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			DesiredCluster:       ctx.CurrentCluster,
			DesiredLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
			Workflow:             te.State,
		}

		// Reconcile k8s cluster to assure new API endpoint has correct certificates.
		if err := u.reconcileK8sCluster(ctx); err != nil {
			return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
		}

		// Patch cluster-info config map to update certificates.
		if err := u.callPatchClusterInfoConfigMap(ctx); err != nil {
			return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
		}

		return ctx.DesiredCluster, ctx.DesiredLoadbalancers, nil
	}

	ctx := &utils.BuilderContext{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Current.K8S,
		DesiredCluster:       te.Event.Task.UpdateState.K8S,
		CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		DesiredLoadbalancers: te.Event.GetTask().GetUpdateState().GetLbs().GetClusters(),
		DeletedLoadBalancers: te.Event.GetTask().GetDeleteState().GetLbs().GetClusters(),
		Workflow:             te.State,
	}

	ctx, err := u.buildCluster(ctx)
	return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
}

func (u *Usecases) executeDeleteTask(te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	if len(te.Event.Task.DeleteState.Nodepools) != 0 {
		k8s, err := u.deleteNodes(te.Current.K8S, te.Event.Task.DeleteState.Nodepools)
		if err != nil {
			return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
		}
		return k8s, te.Current.GetLoadBalancers().GetClusters(), nil
	}

	clusterDeletion := te.Event.Task.DeleteState.GetK8S() != nil

	ctx := &utils.BuilderContext{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Event.Task.DeleteState.GetK8S(),
		CurrentLoadbalancers: te.Event.Task.DeleteState.GetLbs().GetClusters(),
		Workflow:             te.State,
	}

	var err error
	for i := 0; i < maxDeleteRetry; i++ {
		if err = u.destroyCluster(ctx); err == nil {
			if clusterDeletion {
				return nil, nil, nil
			} else {
				currentLbs := te.Current.GetLoadBalancers().GetClusters()
				for _, deleted := range te.Event.Task.DeleteState.GetLbs().GetClusters() {
					currentLbs = slices.DeleteFunc(currentLbs, func(bcluster *spec.LBcluster) bool {
						return deleted.ClusterInfo.Name == bcluster.ClusterInfo.Name
					})
				}
				return te.Current.GetK8S(), currentLbs, nil // TODO: validate
			}
		}
		log.Warn().Msgf("Failed destroying cluster task %q config %q cluster %q: %v", te.Event.Id, te.Config, te.Current.K8S.ClusterInfo.Name, err.Error())
	}
	return nil, nil, err
}
