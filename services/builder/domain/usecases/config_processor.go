package usecases

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	cbox "github.com/berops/claudie/services/context-box/client"
)

const (
	// maxDeleteRetry defines how many times the config should try to be deleted before returning an error, if encountered.
	maxDeleteRetry = 3
)

// ConfigProcessor will fetch new configs from the context-box service. Each received config will be processed in
// a separate go-routine. If a sync.WaitGroup is supplied it will call the Add(1) and then the Done() method on it
// after the go-routine finishes the work, if nil it will be ignored.
func (u *Usecases) ConfigProcessor(wg *sync.WaitGroup) error {
	cboxClient := u.ContextBox.GetClient()
	res, err := cbox.GetConfigBuilder(cboxClient)
	if err != nil {
		return fmt.Errorf("error while getting config from the Context-box: %w", err)
	}

	config := res.GetConfig()
	if config == nil {
		return nil
	}
	wg.Add(1)

	go func() {
		defer wg.Done()

		logger := cutils.CreateLoggerWithProjectName(config.Name)
		clusterView := cutils.NewClusterView(config)

		// If Desired state is null and current is not we delete the infra for the config.
		if config.DsChecksum == nil && config.CsChecksum != nil {
			var err error
			// Try maxDeleteRetry to delete the config.
			for i := 0; i < maxDeleteRetry; i++ {
				logger.Info().Msgf("Destroying config")
				if err = u.destroyConfig(config, clusterView, cboxClient); err == nil {
					// Deletion successful, break here.
					break
				}
			}
			// Save error to DB if not nil.
			if err != nil {
				logger.Err(err).Msg("Error while destroying config")
				if err := utils.SaveConfigWithWorkflowError(config, cboxClient, clusterView); err != nil {
					logger.Err(err).Msg("Failed to save error message")
				}
			}
			return
		}

		// Process each cluster concurrently through the Claudie workflow.
		if err := cutils.ConcurrentExec(clusterView.AllClusters(), func(_ int, clusterName string) error {
			logger := logger.With().Str("cluster", clusterName).Logger()

			// The workflow doesn't handle the case for the deletion of the cluster
			// we need to do this as a separate step.
			if clusterView.DesiredClusters[clusterName] == nil {
				deleteCtx := &utils.BuilderContext{
					ProjectName:          config.Name,
					CurrentCluster:       clusterView.CurrentClusters[clusterName],
					CurrentLoadbalancers: clusterView.DeletedLoadbalancers[clusterName],
					Workflow:             clusterView.ClusterWorkflows[clusterName],
				}

				if err := u.destroyCluster(deleteCtx, cboxClient); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Error while destroying cluster")
					return err
				}

				clusterView.SetWorkflowDone(clusterName)
				logger.Info().Msg("Finished workflow for cluster")
				return u.ContextBox.SaveWorkflowState(config.Name, clusterName, clusterView.ClusterWorkflows[clusterName], cboxClient)
			}

			var (
				// Calculate difference between desired state and current state
				diff = utils.Diff(
					clusterView.CurrentClusters[clusterName],
					clusterView.DesiredClusters[clusterName],
					clusterView.Loadbalancers[clusterName],
					clusterView.DesiredLoadbalancers[clusterName],
				)
				stages       = diff.Stages() + 1 // + 1 as we start indexing from 1.
				currentStage = 0
			)

			// If difference has intermediate representation of states, apply it before real desired state.
			if diff.IR != nil {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				logger.Info().Msgf("Processing stage [%d/%d] for cluster", currentStage, stages)

				ctx := &utils.BuilderContext{
					ProjectName:    config.Name,
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

				if ctx, err = u.buildCluster(ctx, cboxClient); err != nil {
					clusterView.CurrentClusters[clusterName] = ctx.DesiredCluster
					clusterView.Loadbalancers[clusterName] = ctx.DesiredLoadbalancers

					if errors.Is(err, ErrFailedToBuildInfrastructure) {
						clusterView.CurrentClusters[clusterName] = ctx.CurrentCluster
						clusterView.Loadbalancers[clusterName] = ctx.CurrentLoadbalancers
					}

					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}
				logger.Info().Msg("Finished building first stage for cluster")

				// Make the desired state of the temporary cluster the new current state.
				clusterView.CurrentClusters[clusterName] = ctx.DesiredCluster
				// Update nodepool info, as they are not carried over.
				utils.UpdateNodePoolInfo(ctx.DesiredCluster.ClusterInfo.NodePools, clusterView.DesiredClusters[clusterName].ClusterInfo.NodePools)
				clusterView.Loadbalancers[clusterName] = ctx.DesiredLoadbalancers
			}

			// If difference between states replaces control plane, update API endpoint.
			if diff.ControlPlaneWithAPIEndpointReplace {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				logger.Info().Msgf("Processing stage [%d/%d] for cluster", currentStage, stages)

				ctx := &utils.BuilderContext{
					ProjectName:    config.Name,
					CurrentCluster: clusterView.CurrentClusters[clusterName],
					DesiredCluster: clusterView.DesiredClusters[clusterName],
					Workflow:       clusterView.ClusterWorkflows[clusterName],
				}

				if err := u.callUpdateAPIEndpoint(ctx, cboxClient); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}

				clusterView.CurrentClusters[clusterName] = ctx.CurrentCluster
				clusterView.DesiredClusters[clusterName] = ctx.DesiredCluster

				ctx = &utils.BuilderContext{
					ProjectName:          config.Name,
					DesiredCluster:       clusterView.CurrentClusters[clusterName],
					DesiredLoadbalancers: clusterView.Loadbalancers[clusterName],
					Workflow:             clusterView.ClusterWorkflows[clusterName],
				}

				// Reconcile k8s cluster to assure new API endpoint has correct certificates.
				if err := u.reconcileK8sCluster(ctx, cboxClient); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}

				clusterView.CurrentClusters[clusterName] = ctx.DesiredCluster
				clusterView.Loadbalancers[clusterName] = ctx.DesiredLoadbalancers

				// Patch cluster-info config map to update certificates.
				if err := u.callPatchClusterInfoConfigMap(ctx, cboxClient); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}
			}

			// If difference between states results in some nodes being deleted, delete them.
			if len(diff.ToDelete) > 0 {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				logger.Info().Msgf("Processing stage [%d/%d] for cluster", currentStage, stages)

				clusterView.ClusterWorkflows[clusterName].Stage = pb.Workflow_DELETE_NODES
				if err := u.ContextBox.SaveWorkflowState(config.Name, clusterName, clusterView.ClusterWorkflows[clusterName], cboxClient); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					return err
				}
				logger.Info().Msgf("Deleting nodes from cluster")
				cluster, err := u.deleteNodes(clusterView.CurrentClusters[clusterName], clusterView.DesiredClusters[clusterName], diff.ToDelete)
				if err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msgf("Failed to delete nodes")
					return err
				}

				clusterView.CurrentClusters[clusterName] = cluster
			}

			// Apply desired state of the infrastructure after all previous steps (if any required).
			message := "Processing cluster"
			if diff.Stages() > 0 {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				message = fmt.Sprintf("Processing stage [%d/%d] for cluster", currentStage, stages)
			}
			logger.Info().Msgf(message)

			ctx := &utils.BuilderContext{
				ProjectName:          config.Name,
				CurrentCluster:       clusterView.CurrentClusters[clusterName],
				DesiredCluster:       clusterView.DesiredClusters[clusterName],
				CurrentLoadbalancers: clusterView.Loadbalancers[clusterName],
				DesiredLoadbalancers: clusterView.DesiredLoadbalancers[clusterName],
				DeletedLoadBalancers: clusterView.DeletedLoadbalancers[clusterName],
				Workflow:             clusterView.ClusterWorkflows[clusterName],
			}

			if ctx, err = u.buildCluster(ctx, cboxClient); err != nil {
				clusterView.CurrentClusters[clusterName] = ctx.DesiredCluster
				clusterView.Loadbalancers[clusterName] = ctx.DesiredLoadbalancers

				// Save state if error failed to build infrastructure.
				if errors.Is(err, ErrFailedToBuildInfrastructure) {
					clusterView.CurrentClusters[clusterName] = ctx.CurrentCluster
					clusterView.Loadbalancers[clusterName] = ctx.CurrentLoadbalancers
				}

				clusterView.SetWorkflowError(clusterName, err)
				logger.Err(err).Msg("Failed to build cluster")
				return err
			}

			// Propagate the changes made to the cluster back to the View.
			updateFromBuild(ctx, clusterView)

			// If any Loadbalancer are removed, remove them in this step.
			if len(clusterView.DeletedLoadbalancers) > 0 {
				// Perform the deletion of loadbalancers as this won't be handled by the buildCluster Workflow.
				// The BuildInfrastructure in terraformer only performs creation/update for Lbs.
				deleteCtx := &utils.BuilderContext{
					ProjectName:          config.Name,
					CurrentLoadbalancers: clusterView.DeletedLoadbalancers[clusterName],
					Workflow:             clusterView.ClusterWorkflows[clusterName],
				}

				if err := u.destroyCluster(deleteCtx, cboxClient); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Error while destroying cluster")
					return err
				}
			}

			// Workflow finished.
			clusterView.SetWorkflowDone(clusterName)
			if err := u.ContextBox.SaveWorkflowState(config.Name, clusterName, ctx.Workflow, cboxClient); err != nil {
				clusterView.SetWorkflowError(clusterName, err)
				logger.Err(err).Msg("failed to save workflow for cluster")
				return err
			}

			logger.Info().Msg("Finished building cluster")
			return nil
		}); err != nil {
			logger.Err(err).Msg("Error encountered while processing config")

			// Even if the config fails to build, merge the changes as it might be in an in-between state
			// in order to be able to delete it later.
			clusterView.MergeChanges(config)

			if err := utils.SaveConfigWithWorkflowError(config, cboxClient, clusterView); err != nil {
				log.Err(err).Msg("Failed to save error message")
			}
			return
		}

		// Propagate all the changes to the config.
		clusterView.MergeChanges(config)

		// Update the config and store it to the DB.
		logger.Debug().Msg("Saving the config")
		// After successful workflow, set desired state as the current state.
		config.CurrentState = config.DesiredState
		if err := cbox.SaveConfigBuilder(cboxClient, &pb.SaveConfigRequest{Config: config}); err != nil {
			logger.Err(err).Msg("Error while saving the config")
			return
		}
		logger.Info().Msgf("Config finished building")
	}()

	return nil
}

// separateNodepools creates two slices of node names, one for master and one for worker nodes
func separateNodepools(clusterNodes map[string]int32, currentClusterInfo, desiredClusterInfo *pb.ClusterInfo) (master []string, worker []string) {
	for _, nodepool := range currentClusterInfo.NodePools {
		var names = make([]string, 0, len(nodepool.Nodes))
		if np := nodepool.GetDynamicNodePool(); np != nil {
			if count, ok := clusterNodes[nodepool.Name]; ok && count > 0 {
				names = getDynamicNodeNames(nodepool, int(count))
			}
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			if count, ok := clusterNodes[nodepool.Name]; ok && count > 0 {
				names = getStaticNodeNames(nodepool, desiredClusterInfo)
			}
		}
		if nodepool.IsControl {
			master = append(master, names...)
		} else {
			worker = append(worker, names...)
		}
	}
	return master, worker
}

// getDynamicNodeNames returns slice of length count with names of the nodes from specified nodepool
// nodes chosen are from the last element in Nodes slice, up to the first one
func getDynamicNodeNames(np *pb.NodePool, count int) (names []string) {
	for i := len(np.GetNodes()) - 1; i >= len(np.GetNodes())-count; i-- {
		names = append(names, np.GetNodes()[i].GetName())
		log.Debug().Msgf("Choosing node %s for deletion", np.GetNodes()[i].GetName())
	}
	return names
}

// getStaticNodeNames returns slice of length count with names of the nodes from specified nodepool
// nodes chosen are from the last element in Nodes slice, up to the first one
func getStaticNodeNames(np *pb.NodePool, desiredCluster *pb.ClusterInfo) (names []string) {
	// Find desired nodes for node pool.
	desired := make(map[string]struct{})
	for _, n := range desiredCluster.NodePools {
		if n.Name == np.Name {
			for _, node := range n.Nodes {
				desired[node.Name] = struct{}{}
			}
		}
	}
	// Find deleted nodes
	if n := np.GetStaticNodePool(); n != nil {
		for _, node := range np.Nodes {
			if _, ok := desired[node.Name]; !ok {
				// Append name as it is not defined in desired state.
				names = append(names, node.Name)
			}
		}
	}
	return names
}

// deleteNodes deletes nodes from the cluster based on the node map specified.
func (u *Usecases) deleteNodes(currentCluster, desiredCluster *pb.K8Scluster, nodes map[string]int32) (*pb.K8Scluster, error) {
	master, worker := separateNodepools(nodes, currentCluster.ClusterInfo, desiredCluster.ClusterInfo)
	newCluster, err := u.callDeleteNodes(master, worker, currentCluster)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes for %s : %w", currentCluster.ClusterInfo.Name, err)
	}

	return newCluster, nil
}

// updateFromBuild takes the changes after a successful workflow of a given cluster
func updateFromBuild(ctx *utils.BuilderContext, view *cutils.ClusterView) {
	if ctx.CurrentCluster != nil {
		view.CurrentClusters[ctx.CurrentCluster.ClusterInfo.Name] = ctx.CurrentCluster
	}

	if ctx.DesiredCluster != nil {
		view.DesiredClusters[ctx.DesiredCluster.ClusterInfo.Name] = ctx.DesiredCluster
	}

	if ctx.Workflow != nil {
		view.ClusterWorkflows[ctx.GetClusterName()] = ctx.Workflow
	}

	for _, current := range ctx.CurrentLoadbalancers {
		for i := range view.Loadbalancers[current.TargetedK8S] {
			if view.Loadbalancers[current.TargetedK8S][i].ClusterInfo.Name == current.ClusterInfo.Name {
				view.Loadbalancers[current.TargetedK8S][i] = current
				break
			}
		}
	}

	for _, desired := range ctx.DesiredLoadbalancers {
		for i := range view.DesiredLoadbalancers[desired.TargetedK8S] {
			if view.DesiredLoadbalancers[desired.TargetedK8S][i].ClusterInfo.Name == desired.ClusterInfo.Name {
				view.DesiredLoadbalancers[desired.TargetedK8S][i] = desired
				break
			}
		}
	}

	for _, deleted := range ctx.DeletedLoadBalancers {
		for i := range view.DeletedLoadbalancers[deleted.TargetedK8S] {
			if view.DeletedLoadbalancers[deleted.TargetedK8S][i].ClusterInfo.Name == deleted.ClusterInfo.Name {
				view.DeletedLoadbalancers[deleted.TargetedK8S][i] = deleted
				break
			}
		}
	}
}
