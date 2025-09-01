package usecases

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/metrics"
	builder "github.com/berops/claudie/services/builder/internal"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
)

func (u *Usecases) TaskProcessor(ctx context.Context, wg *sync.WaitGroup) error {
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

		updatedState, err := u.processTaskEvent(ctx, task)
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

func (u *Usecases) processTaskEvent(ctx context.Context, t *managerclient.NextTaskResponse) (*spec.Clusters, error) {
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
		k8s, lbs, err = u.executeCreateTask(ctx, t)
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
		k8s, lbs, err = u.executeUpdateTask(ctx, t)
		if err != nil {
			metrics.ClustersUpdated.Inc()
		}
	case spec.Event_DELETE:
		metrics.TasksProcessedDeleteCounter.Inc()
		metrics.ClusterProcessedCounter.Inc()
		metrics.LoadBalancersProcessedCounter.Add(float64(len(t.Event.Task.DeleteState.GetLbs())))
		metrics.LoadBalancersInProgress.Add(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		defer metrics.LoadBalancersInProgress.Sub(float64(len(t.Event.Task.CreateState.GetLbs().GetClusters())))
		metrics.ClustersInDelete.Inc()
		defer metrics.ClustersInDelete.Dec()
		log.Debug().Msgf("[task %q] Delete operation %q from config %q", t.Event.Id, t.Cluster, t.Config)
		k8s, lbs, err = u.executeDeleteTask(ctx, t)
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

func (u *Usecases) executeCreateTask(ctx context.Context, te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	work := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		DesiredCluster:       te.Event.Task.CreateState.K8S,
		DesiredLoadbalancers: te.Event.Task.CreateState.GetLbs().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}

	logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
	err := u.buildCluster(ctx, work, &logger)
	return work.DesiredCluster, work.DesiredLoadbalancers, err
}

func (u *Usecases) executeUpdateTask(ctx context.Context, te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	if te.Event.Task.UpdateState.EndpointChange != nil {
		work := &builder.Context{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			CurrentCluster:       te.Current.K8S,
			CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
			Workflow:             te.State,
			Options:              te.Event.Task.Options,
		}
		logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)

		switch typ := te.Event.Task.UpdateState.EndpointChange.(type) {
		case *spec.UpdateState_LbEndpointChange:
			cid := typ.LbEndpointChange.CurrentEndpointId
			did := typ.LbEndpointChange.DesiredEndpointId
			stt := typ.LbEndpointChange.State
			err := u.tryProcessTask(ctx, work, &logger, u.determineApiEndpointChange(cid, did, stt))
			if err != nil {
				return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
			}
		case *spec.UpdateState_NewControlEndpoint:
			np := typ.NewControlEndpoint.Nodepool
			n := typ.NewControlEndpoint.Node
			err := u.tryProcessTask(ctx, work, &logger, u.updateControlPlaneApiEndpoint(np, n))
			if err != nil {
				return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
			}
		}

		work = &builder.Context{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			DesiredCluster:       work.CurrentCluster,
			DesiredLoadbalancers: work.CurrentLoadbalancers,
			Workflow:             te.State,
			Options:              te.Event.Task.Options,
		}
		logger = loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
		tasks := []Task{
			{
				do:          u.reconcileK8sCluster,
				stage:       spec.Workflow_KUBE_ELEVEN,
				description: "reconciling cluster after API endpoint change",
			},
			{
				do:          u.patchConfigMapsWithNewApiEndpoint,
				stage:       spec.Workflow_KUBER,
				description: "reconciling cluster configuration after API endpoint change",
			},
			{
				do:          u.patchKubeadmAndUpdateCilium,
				stage:       spec.Workflow_KUBER,
				description: "reconciling cluster configuration after API endpoint change",
			},
			{
				do: func(ctx context.Context, work *builder.Context, logger *zerolog.Logger) error {
					return u.Kuber.GpuOperatorRolloutRestart(work.DesiredCluster, u.Kuber.GetClient())
				},
				stage:           spec.Workflow_KUBER,
				description:     "performing rollout of NVIDIA container toolkit, if present",
				continueOnError: true,
			},
		}

		if err := u.processTasks(ctx, work, &logger, tasks); err != nil {
			return work.DesiredCluster, work.DesiredLoadbalancers, err
		}
		return work.DesiredCluster, work.DesiredLoadbalancers, nil
	}

	work := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Current.K8S,
		DesiredCluster:       te.Event.Task.UpdateState.K8S,
		CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		DesiredLoadbalancers: te.Event.GetTask().GetUpdateState().GetLbs().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}

	logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
	err := u.buildCluster(ctx, work, &logger)
	return work.DesiredCluster, work.DesiredLoadbalancers, err
}

func (u *Usecases) executeDeleteTask(ctx context.Context, te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	if te.Event.Task.DeleteState.K8S != nil {
		if te.Event.Task.DeleteState.K8S.Destroy {
			work := &builder.Context{
				ProjectName:          te.Config,
				TaskId:               te.Event.Id,
				CurrentCluster:       te.Current.K8S,
				CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
				Workflow:             te.State,
				Options:              te.Event.Task.Options,
			}
			logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
			if err := u.destroyCluster(ctx, work, &logger); err != nil {
				return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), err
			}
			return nil, nil, nil
		}
		if len(te.Event.Task.DeleteState.K8S.Nodepools) > 0 {
			return u.deleteK8sNodes(ctx, te)
		}
	}

	var deleted []*spec.LBcluster
	for _, lb := range te.Event.Task.DeleteState.Lbs {
		i := clusters.IndexLoadbalancerById(lb.Id, te.Current.GetLoadBalancers().GetClusters())
		if i < 0 {
			continue
		}
		if lb.Destroy {
			deleted = append(deleted, te.Current.GetLoadBalancers().GetClusters()[i])
		}
	}

	if len(deleted) > 0 {
		work := &builder.Context{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			CurrentLoadbalancers: deleted,
			Workflow:             te.State,
			Options:              te.Event.Task.Options,
		}
		logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
		err := u.tryProcessTask(ctx, work, &logger, Task{
			do:          u.destroyInfrastructure,
			stage:       spec.Workflow_TERRAFORMER,
			description: "deleting loadbalancer infrastructure",
		})
		if err != nil {
			return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), err
		}
	}

	currentLbs := spec.LoadBalancers{Clusters: te.Current.GetLoadBalancers().GetClusters()}
	for _, deleted := range deleted {
		currentLbs.Clusters = slices.DeleteFunc(currentLbs.Clusters, func(bcluster *spec.LBcluster) bool { return deleted.ClusterInfo.Id() == bcluster.ClusterInfo.Id() })
	}

	lbs := proto.Clone(&currentLbs).(*spec.LoadBalancers)
	var npsDeleted bool
	for _, lb := range te.Event.Task.DeleteState.Lbs {
		i := clusters.IndexLoadbalancerById(lb.Id, lbs.Clusters)
		if i < 0 || len(lb.Nodepools) < 1 {
			continue
		}
		npsDeleted = true
		for np := range lb.Nodepools {
			lbs.Clusters[i].ClusterInfo.NodePools = nodepools.DeleteByName(lbs.Clusters[i].ClusterInfo.NodePools, np)
		}
	}

	if !npsDeleted {
		return te.Current.K8S, currentLbs.Clusters, nil
	}

	work := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Current.K8S,
		DesiredCluster:       te.Current.K8S,
		CurrentLoadbalancers: currentLbs.Clusters,
		DesiredLoadbalancers: lbs.Clusters,
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}

	logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
	err := u.tryProcessTask(ctx, work, &logger, Task{
		do:          u.reconcileInfrastructure,
		stage:       spec.Workflow_TERRAFORMER,
		description: "reconciling loadbalancer nodepools",
	})
	if err != nil {
		return te.Current.K8S, currentLbs.Clusters, err
	}

	return te.Current.K8S, lbs.Clusters, nil
}

func (u *Usecases) deleteK8sNodes(ctx context.Context, te *managerclient.NextTaskResponse) (*spec.K8Scluster, []*spec.LBcluster, error) {
	var (
		staticCount  int
		dynamicCount int
		static       []*spec.NodePool
	)

	for np, deleted := range te.Event.Task.DeleteState.K8S.Nodepools {
		if np := nodepools.FindByName(np, te.Current.K8S.ClusterInfo.NodePools); np.GetStaticNodePool() != nil {
			static = append(static, proto.Clone(np).(*spec.NodePool))
			staticCount += len(deleted.Nodes)
		} else {
			dynamicCount += len(deleted.Nodes)
		}
	}

	deleteWork := &builder.Context{
		ProjectName:    te.Config,
		TaskId:         te.Event.Id,
		CurrentCluster: te.Current.K8S,
		Workflow:       te.State,
		Options:        te.Event.Task.Options,
	}
	logger := loggerutils.WithTaskContext(deleteWork.ProjectName, deleteWork.Id(), deleteWork.TaskId)
	err := u.tryProcessTask(ctx, deleteWork, &logger, u.deleteNodesFromCurrentState(te.Event.Task.DeleteState.K8S.Nodepools, staticCount, dynamicCount))
	if err != nil {
		return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while deleting nodes for %s: %w", te.Current.K8S.ClusterInfo.Name, err)
	}

	k8sAfterNodeDeletion := deleteWork.CurrentCluster

	// for static nodes de-initialize them by removing installed binaries.
	if staticCount != 0 {
		// for static nodes we need to delete installed claudie utilities.
		for _, np := range static {
			np.Nodes = slices.DeleteFunc(np.Nodes, func(node *spec.Node) bool {
				return !slices.ContainsFunc(te.Event.Task.DeleteState.K8S.Nodepools[np.Name].Nodes, func(s string) bool {
					return node.Name == s
				})
			})
		}

		c := proto.Clone(te.Current.K8S).(*spec.K8Scluster)
		c.ClusterInfo.NodePools = static

		work := &builder.Context{
			ProjectName:    te.Config,
			TaskId:         te.Event.Id,
			CurrentCluster: c,
			Workflow:       te.State,
			Options:        te.Event.Task.Options,
		}

		logger := loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)
		err := u.tryProcessTask(ctx, work, &logger, Task{
			do:          u.removeClaudieUtilities,
			stage:       spec.Workflow_ANSIBLER,
			description: "removing claudie utilities from static nodes, after deletion",
		})
		if err != nil {
			// We do not return on an error here, as the nodes are deleted from the cluster, even if some of the claudie installed
			// utilities would failed to be removed, for example if one of the static nodes would become unreachable during the process we
			// want to continue with the next step.
			logger.Warn().Msgf("error while removing utilities for static nodes from %s: %v, continuing", te.Current.K8S.ClusterInfo.Id(), err)
		}
	}

	work := &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Current.K8S,
		DesiredCluster:       k8sAfterNodeDeletion,
		CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		DesiredLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}
	logger = loggerutils.WithTaskContext(work.ProjectName, work.Id(), work.TaskId)

	tasks := []Task{
		{
			do: func(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
				return u.Kuber.StoreClusterMetadata(work, u.Kuber.GetClient())
			},
			stage:           spec.Workflow_KUBER,
			description:     "updating cluster metadata secret, after node deletion",
			continueOnError: true,
		},
		{
			do:          u.reconcileInfrastructure,
			stage:       spec.Workflow_TERRAFORMER,
			description: "reconciling dynamic nodepools infrastructure, after node deletion",
			condition: func(_ *builder.Context) bool {
				// reconcile the dynamic nodes only if we actually deleted dynamic nodes.
				return dynamicCount != 0
			},
		},
		// After removing the nodes, we need to run the new current state through ansibler to remove the existing VPNs connections to these nodes
		// and update the VPN configs. We can ignore the kube-eleven step ( as the nodes already are deleted from the k8s cluster), in the kuber stage no
		// patching needs to be done, only update of the kubeadm config map.
		{
			do:          u.configureInfrastructure,
			stage:       spec.Workflow_ANSIBLER,
			description: "configuring infrastructure, after deletion",
		},
		{
			do:          u.patchKubeadmAndUpdateCilium,
			stage:       spec.Workflow_KUBER,
			description: "reconciling cluster configuration, after node deletion",
		},
		// The daemonset for the NVIDIA toolkit does not need to be restarted here as kube_eleven
		// is not run.
	}

	if err := u.processTasks(ctx, work, &logger, tasks); err != nil {
		// At this point we can't return the current state k8s cluster as nodes have been deleted from the cluster itself
		// and potentionally also from the terraform state (i.e the public IPs were de-allocated) thus we also need to communicate
		// back any partial state that was applied for the k8s cluster.
		return work.DesiredCluster, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while configuring infrastructure after node deletion from %s: %w", te.Current.K8S.ClusterInfo.Id(), err)
	}

	u.updateTaskWithDescription(work, spec.Workflow_KUBER, fmt.Sprintf("finished deleting nodes from cluster static: %v,dynamic: %v", staticCount, dynamicCount))

	// work.DesiredCluster will return the current cluster after the node deletion and other changes, if any.
	return work.DesiredCluster, te.Current.GetLoadBalancers().GetClusters(), nil
}
