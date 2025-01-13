package usecases

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/metrics"
	builder "github.com/berops/claudie/services/builder/internal"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
)

func (u *Usecases) TaskProcessor(wg *sync.WaitGroup) error {
	ctx := context.Background()

	task, err := u.Manager.NextTask(ctx)
	if err != nil || task == nil {
		if errors.Is(err, managerclient.ErrVersionMismatch) {
			log.Debug().Msgf("failed to receive next task due to a dirty write")
		}
		if !errors.Is(err, managerclient.ErrNotFound) {
			return err
		}
		return nil
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		updatedState, err := u.processTaskEvent(task)
		if err != nil {
			metrics.TasksProcessedErrCounter.Inc()
			log.Err(err).Msgf("failed to process task %q for cluster %q for config %q", task.Event.Id, task.Cluster, task.Config)
			task.State.Status = spec.Workflow_ERROR
			task.State.Description = err.Error()
			// fallthrough
		} else {
			metrics.TasksProcessedOkCounter.Inc()
			log.Info().Msgf("successfully processed task %q for cluster %q for config %q", task.Event.Id, task.Cluster, task.Config)
			task.State.Status = spec.Workflow_DONE
			task.State.Stage = spec.Workflow_NONE
			task.State.Description = "Finished successfully"
			// fallthrough
		}

		err = managerclient.Retry(&log.Logger, fmt.Sprintf("Completing task %q", task.Event.Id), func() error {
			log.Debug().Msgf("completing task %q for cluster %q for config %q with status: %s", task.Event.Id, task.Cluster, task.Config, task.State.Status.String())
			err := u.Manager.TaskComplete(ctx, &managerclient.TaskCompleteRequest{
				Config:   task.Config,
				Cluster:  task.Cluster,
				TaskId:   task.Event.Id,
				Workflow: task.State,
				State:    updatedState,
			})
			if errors.Is(err, managerclient.ErrNotFound) {
				log.Warn().Msgf("can't complete task %q from config %q cluster %q: %v", task.Event.Id, task.Config, task.Cluster, err)
			}
			return err
		})
		if err != nil {
			log.Err(err).Msgf("failed to mark task %q from cluster %q config %q, as completed", task.Event.Id, task.Cluster, task.Config)
		}
		log.Info().Msgf("Finished processing task %q for cluster %q config %q", task.Event.Id, task.Cluster, task.Config)
	}()
	return nil
}

func (u *Usecases) processTaskEvent(t *managerclient.NextTaskResponse) (*spec.Clusters, error) {
	metrics.TasksProcessedCounter.Inc()

	t.State.Description = fmt.Sprintf("%s:", t.Event.Description)

	var (
		err error
		k8s *spec.K8Scluster
		lbs []*spec.LBcluster
	)

	metrics.ClustersInProgress.Inc()
	defer metrics.ClustersInProgress.Dec()

	switch t.Event.Event {
	case spec.Event_CREATE:
		metrics.TasksProcessedCreateCounter.Inc()
		metrics.ClusterProcessedCounter.Inc()
		metrics.LoadBalancersProcessedCounter.Add(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		metrics.LoadBalancersInProgress.Add(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		defer metrics.LoadBalancersInProgress.Sub(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		metrics.ClustersInCreate.Inc()
		defer metrics.ClustersInCreate.Dec()
		log.Debug().Msgf("[task %q] Create operation cluster %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeCreateTask(t)
		if err != nil {
			metrics.ClustersCreated.Inc()
		}
	case spec.Event_UPDATE:
		metrics.TasksProcessedUpdateCounter.Inc()
		metrics.ClusterProcessedCounter.Inc()
		metrics.LoadBalancersProcessedCounter.Add(float64(len(t.Event.Task.UpdateState.GetLbs().GetClusters())))
		metrics.LoadBalancersInProgress.Add(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		defer metrics.LoadBalancersInProgress.Sub(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		metrics.ClustersInUpdate.Inc()
		defer metrics.ClustersInUpdate.Dec()
		log.Debug().Msgf("[task %q] Update operation %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeUpdateTask(t)
		if err != nil {
			metrics.ClustersUpdated.Inc()
		}
	case spec.Event_DELETE:
		metrics.TasksProcessedDeleteCounter.Inc()
		metrics.ClusterProcessedCounter.Inc()
		metrics.LoadBalancersProcessedCounter.Add(float64(len(t.Event.Task.DeleteState.GetLbs().GetClusters())))
		metrics.LoadBalancersInProgress.Add(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		defer metrics.LoadBalancersInProgress.Sub(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		metrics.ClustersInDelete.Inc()
		defer metrics.ClustersInDelete.Dec()
		log.Debug().Msgf("[task %q] Delete operation %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeDeleteTask(t)
		if err != nil {
			metrics.ClustersDeleted.Inc()
		}
	}

	// even on error we construct the current state
	// as changes could have been done in steps that
	// succeeded.
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
	ctx := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		DesiredCluster:       te.Event.Task.CreateState.K8S,
		DesiredLoadbalancers: te.Event.Task.CreateState.GetLbs().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}
	ctx, err := u.buildCluster(ctx)
	return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
}

func (u *Usecases) executeUpdateTask(te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	if te.Event.Task.UpdateState.EndpointChange != nil {
		ctx := &builder.Context{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			CurrentCluster:       te.Current.K8S,
			CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
			Workflow:             te.State,
			Options:              te.Event.Task.Options,
		}

		switch typ := te.Event.Task.UpdateState.EndpointChange.(type) {
		case *spec.UpdateState_LbEndpointChange:
			cid := typ.LbEndpointChange.CurrentEndpointId
			did := typ.LbEndpointChange.DesiredEndpointId
			stt := typ.LbEndpointChange.State
			if err := u.determineApiEndpointChange(ctx, cid, did, stt); err != nil {
				return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
			}
		case *spec.UpdateState_NewControlEndpoint:
			np := typ.NewControlEndpoint.Nodepool
			n := typ.NewControlEndpoint.Node

			if err := u.callUpdateAPIEndpoint(ctx, np, n); err != nil {
				return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
			}
		}

		ctx = &builder.Context{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			DesiredCluster:       ctx.CurrentCluster,
			DesiredLoadbalancers: ctx.CurrentLoadbalancers,
			Workflow:             te.State,
			Options:              te.Event.Task.Options,
		}

		// Reconcile k8s cluster to assure new API endpoint has correct certificates.
		if err := u.reconcileK8sCluster(ctx); err != nil {
			return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
		}

		// Patch cluster-info config map to update certificates.
		if err := u.callPatchClusterInfoConfigMap(ctx); err != nil {
			return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
		}

		if err := u.Kuber.CiliumRolloutRestart(ctx.DesiredCluster, u.Kuber.GetClient()); err != nil {
			return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
		}

		return ctx.DesiredCluster, ctx.DesiredLoadbalancers, nil
	}

	ctx := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Current.K8S,
		DesiredCluster:       te.Event.Task.UpdateState.K8S,
		CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		DesiredLoadbalancers: te.Event.GetTask().GetUpdateState().GetLbs().GetClusters(),
		DeletedLoadBalancers: te.Event.GetTask().GetDeleteState().GetLbs().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}

	ctx, err := u.buildCluster(ctx)
	return ctx.DesiredCluster, ctx.DesiredLoadbalancers, err
}

func (u *Usecases) executeDeleteTask(te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	if len(te.Event.Task.DeleteState.Nodepools) != 0 {
		var static []*spec.NodePool
		var master, worker []string

		for np, deleted := range te.Event.Task.DeleteState.Nodepools {
			nodepool := nodepools.FindByName(np, te.Current.K8S.ClusterInfo.NodePools)
			if nodepool.IsControl {
				master = append(master, deleted.Nodes...)
			} else {
				worker = append(worker, deleted.Nodes...)
			}
			if nodepool.GetStaticNodePool() != nil {
				static = append(static, proto.Clone(nodepool).(*spec.NodePool))
			}
		}

		ctx := &builder.Context{
			ProjectName:    te.Config,
			TaskId:         te.Event.Id,
			CurrentCluster: te.Current.K8S,
			Workflow:       te.State,
			Options:        te.Event.Task.Options,
		}

		u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("deleting nodes [%q, %q] from cluster", master, worker))

		k8s, err := u.callDeleteNodes(master, worker, ctx.CurrentCluster)
		if err != nil {
			return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while deleting nodes for %s: %w", te.Current.K8S.ClusterInfo.NodePools, err)
		}

		if len(static) == 0 {
			u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("finished deleting nodes [%q, %q] from cluster", master, worker))
			return k8s, te.Current.GetLoadBalancers().GetClusters(), nil
		}

		// for static nodes we need to delete installed claudie utilities.
		for _, np := range static {
			np.Nodes = slices.DeleteFunc(np.Nodes, func(node *spec.Node) bool {
				return !slices.ContainsFunc(te.Event.Task.DeleteState.Nodepools[np.Name].Nodes, func(s string) bool {
					return node.Name == s
				})
			})
		}

		c := proto.Clone(te.Current.K8S).(*spec.K8Scluster)
		c.ClusterInfo.NodePools = static

		ctx = &builder.Context{
			ProjectName:    te.Config,
			TaskId:         te.Event.Id,
			CurrentCluster: c,
			Workflow:       te.State,
			Options:        te.Event.Task.Options,
		}

		if err := u.removeClaudieUtilities(ctx); err != nil {
			return k8s, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while removing utilities for static nodes from %s: %w", te.Current.K8S.ClusterInfo.Name, err)
		}

		u.updateTaskWithDescription(ctx, spec.Workflow_KUBER, fmt.Sprintf("finished deleting nodes [%q, %q] from cluster", master, worker))
		return k8s, te.Current.GetLoadBalancers().GetClusters(), nil
	}

	clusterDeletion := te.Event.Task.DeleteState.GetK8S() != nil

	ctx := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Event.Task.DeleteState.GetK8S(),
		CurrentLoadbalancers: te.Event.Task.DeleteState.GetLbs().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}

	err := u.destroyCluster(ctx)
	if err == nil {
		if clusterDeletion {
			return nil, nil, nil
		} else {
			currentLbs := te.Current.GetLoadBalancers().GetClusters()
			for _, deleted := range te.Event.Task.DeleteState.GetLbs().GetClusters() {
				currentLbs = slices.DeleteFunc(currentLbs, func(bcluster *spec.LBcluster) bool {
					return deleted.ClusterInfo.Name == bcluster.ClusterInfo.Name
				})
			}
			return te.Current.GetK8S(), currentLbs, nil
		}
	}

	log.Warn().Msgf("Failed destroying cluster task %q config %q cluster %q: %v", te.Event.Id, te.Config, te.Current.K8S.ClusterInfo.Name, err.Error())
	return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), err
}
