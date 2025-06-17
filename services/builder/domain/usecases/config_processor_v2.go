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

	logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
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

		switch typ := te.Event.Task.UpdateState.EndpointChange.(type) {
		case *spec.UpdateState_LbEndpointChange:
			cid := typ.LbEndpointChange.CurrentEndpointId
			did := typ.LbEndpointChange.DesiredEndpointId
			stt := typ.LbEndpointChange.State
			if err := u.determineApiEndpointChange(work, cid, did, stt); err != nil {
				return te.Current.GetK8S(), te.Current.GetLoadBalancers().GetClusters(), err
			}
		case *spec.UpdateState_NewControlEndpoint:
			np := typ.NewControlEndpoint.Nodepool
			n := typ.NewControlEndpoint.Node

			if err := u.callUpdateAPIEndpoint(work, np, n); err != nil {
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

		logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())

		if err := u.reconcileK8sCluster(ctx, work, &logger); err != nil {
			return work.DesiredCluster, work.DesiredLoadbalancers, err
		}

		if err := u.patchConfigMapsWithNewApiEndpoint(ctx, work, &logger); err != nil {
			return work.DesiredCluster, work.DesiredLoadbalancers, err
		}

		if err := u.patchKubeadmAndUpdateCilium(ctx, work, &logger); err != nil {
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

	logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
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
			logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
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
		logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
		if err := u.destroyInfrastructure(ctx, work, &logger); err != nil {
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

	logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
	if err := u.reconcileInfrastructure(ctx, work, &logger); err != nil {
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

	work := &builder.Context{
		ProjectName:    te.Config,
		TaskId:         te.Event.Id,
		CurrentCluster: te.Current.K8S,
		Workflow:       te.State,
		Options:        te.Event.Task.Options,
	}
	u.updateTaskWithDescription(work, spec.Workflow_KUBER, fmt.Sprintf("deleting nodes from cluster static:%v,dynamic:%v ", staticCount, dynamicCount))

	// delete the nodes from the k8s cluster.
	k8s, err := u.callDeleteNodes(te.Current.K8S, te.Event.Task.DeleteState.K8S.Nodepools)
	if err != nil {
		return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while deleting nodes for %s: %w", te.Current.K8S.ClusterInfo.Name, err)
	}

	// for dynamic nodes remove the infrastructure via terraform.
	if dynamicCount != 0 {
		work := &builder.Context{
			ProjectName:          te.Config,
			TaskId:               te.Event.Id,
			CurrentCluster:       te.Current.K8S,
			DesiredCluster:       k8s,
			CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
			DesiredLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
			Workflow:             te.State,
			Options:              te.Event.Task.Options,
		}

		logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
		if err := u.reconcileInfrastructure(ctx, work, &logger); err != nil {
			return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while deleting nodes for %s: %w", te.Current.K8S.ClusterInfo.Id(), err)
		}
	}

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

		logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
		if err := u.removeClaudieUtilities(ctx, work, &logger); err != nil {
			return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while removing utilities for static nodes from %s: %w", te.Current.K8S.ClusterInfo.Id(), err)
		}
	}

	// After removing the nodes, we need to run the new current state through ansibler to remove the existing VPNs connections to these nodes.and Update ansibler VPN.
	//  We can ignore the kube-eleven step (the nodes were already deleted from the k8s cluster), the kuber stage no patching of nodes needs to be done only updade of the kubeadm config map..
	work = &builder.Context{
		ProjectName:          te.Config,
		TaskId:               te.Event.Id,
		CurrentCluster:       te.Current.K8S,
		DesiredCluster:       k8s,
		CurrentLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		DesiredLoadbalancers: te.Current.GetLoadBalancers().GetClusters(),
		Workflow:             te.State,
		Options:              te.Event.Task.Options,
	}

	logger := loggerutils.WithProjectAndCluster(work.ProjectName, work.Id())
	if err := u.configureInfrastructure(ctx, work, &logger); err != nil {
		return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while configuring infrastructure after node deletion from %s: %w", te.Current.K8S.ClusterInfo.Id(), err)
	}

	if err := u.patchKubeadmAndUpdateCilium(ctx, work, &logger); err != nil {
		return te.Current.K8S, te.Current.GetLoadBalancers().GetClusters(), fmt.Errorf("error while configuring infrastructure after node deletion from %s: %w", te.Current.K8S.ClusterInfo.Id(), err)
	}

	u.updateTaskWithDescription(work, spec.Workflow_KUBER, fmt.Sprintf("finished deleting nodes from cluster static%v,dynamic%v", staticCount, dynamicCount))
	return k8s, te.Current.GetLoadBalancers().GetClusters(), nil
}
