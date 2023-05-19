package main

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
)

const (
	// maxDeleteRetry defines how many times the config should try to be deleted before returning an error, if encountered.
	maxDeleteRetry = 3
)

// configProcessor will fetch new configs from the context-box service. Each received config will be processed in
// a separate go-routine. If a sync.WaitGroup is supplied it will call the Add(1) and then the Done() method on it
// after the go-routine finishes the work, if nil it will be ignored.
func configProcessor(c pb.ContextBoxServiceClient, wg *sync.WaitGroup) error {
	res, err := cbox.GetConfigBuilder(c) // Get a new config
	if err != nil {
		return fmt.Errorf("error while getting config from the Context-box: %w", err)
	}

	config := res.GetConfig()
	if config == nil {
		return nil
	}

	if wg != nil {
		// we received a non-nil config thus we add a new worker to the wait group.
		wg.Add(1)
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}

		logger := utils.CreateLoggerWithProjectName(config.Name)

		clusterView := NewClusterView(config)

		// if Desired state is null and current is not we delete the infra for the config.
		if config.DsChecksum == nil && config.CsChecksum != nil {
			var err error
			// Try maxDeleteRetry to delete the config.
			for i := 0; i < maxDeleteRetry; i++ {
				logger.Info().Msgf("Destroying config")
				if err = destroyConfig(config, clusterView, c); err == nil {
					// Deletion successful, break here.
					break
				}
			}
			// Save error to DB if not nil.
			if err != nil {
				logger.Err(err).Msg("Error while destroying config")
				if err := saveConfigWithWorkflowError(config, c, clusterView); err != nil {
					logger.Err(err).Msg("Failed to save error message")
				}
			}
			return
		}

		if err := utils.ConcurrentExec(clusterView.AllClusters(), func(clusterName string) error {
			logger := logger.With().Str("cluster", clusterName).Logger()

			// The workflow doesn't handle the case for the deletion of the cluster
			// we need to do this as a separate step.
			if clusterView.DesiredClusters[clusterName] == nil {
				deleteCtx := &BuilderContext{
					projectName:   config.Name,
					cluster:       clusterView.CurrentClusters[clusterName],
					loadbalancers: clusterView.DeletedLoadbalancers[clusterName],
					Workflow:      clusterView.ClusterWorkflows[clusterName],
				}

				if err := destroyCluster(deleteCtx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Error while destroying cluster")
					return err
				}

				clusterView.SetWorkflowDone(clusterName)
				logger.Info().Msg("Finished workflow for cluster")
				return updateWorkflowStateInDB(config.Name, clusterName, clusterView.ClusterWorkflows[clusterName], c)
			}

			var (
				diff = Diff(
					clusterView.CurrentClusters[clusterName],
					clusterView.DesiredClusters[clusterName],
					clusterView.Loadbalancers[clusterName],
					clusterView.DesiredLoadbalancers[clusterName],
				)
				stages       = diff.Stages() + 1 // + 1 as we start indexing from 1.
				currentStage = 0
			)

			if diff.IR != nil {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				logger.Info().Msgf("Processing stage [%d/%d] for cluster", currentStage, stages)

				ctx := &BuilderContext{
					projectName:    config.Name,
					cluster:        clusterView.CurrentClusters[clusterName],
					desiredCluster: diff.IR,

					// If there are any Lbs for the current state keep them.
					// Ignore the desired state for the Lbs for now. Use the
					// current state for desired to not trigger any changes.
					// as we only care about addition of nodes in this step.
					loadbalancers:        clusterView.Loadbalancers[clusterName],
					desiredLoadbalancers: clusterView.Loadbalancers[clusterName],
					deletedLoadBalancers: nil,

					Workflow: clusterView.ClusterWorkflows[clusterName],
				}

				if ctx, err = buildCluster(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}
				logger.Info().Msg("Finished building first stage for cluster")

				// make the desired state of the temporary cluster the new current state.
				clusterView.CurrentClusters[clusterName] = ctx.desiredCluster
				clusterView.Loadbalancers[clusterName] = ctx.desiredLoadbalancers
			}

			if diff.ControlPlaneWithAPIEndpointReplace {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				logger.Info().Msgf("Processing stage [%d/%d] for cluster", currentStage, stages)

				ctx := &BuilderContext{
					projectName:    config.Name,
					cluster:        clusterView.CurrentClusters[clusterName],
					desiredCluster: clusterView.DesiredClusters[clusterName],
					Workflow:       clusterView.ClusterWorkflows[clusterName],
				}

				if err := callUpdateAPIEndpoint(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}

				clusterView.CurrentClusters[clusterName] = ctx.cluster
				clusterView.DesiredClusters[clusterName] = ctx.desiredCluster

				ctx = &BuilderContext{
					projectName:          config.Name,
					desiredCluster:       clusterView.CurrentClusters[clusterName],
					desiredLoadbalancers: clusterView.Loadbalancers[clusterName],
					Workflow:             clusterView.ClusterWorkflows[clusterName],
				}

				if err := callKubeEleven(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}

				clusterView.CurrentClusters[clusterName] = ctx.desiredCluster
				clusterView.Loadbalancers[clusterName] = ctx.desiredLoadbalancers

				if err := callPatchClusterInfoConfigMap(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Failed to build cluster")
					return err
				}
			}

			if len(diff.ToDelete) > 0 {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				logger.Info().Msgf("Processing stage [%d/%d] for cluster", currentStage, stages)

				clusterView.ClusterWorkflows[clusterName].Stage = pb.Workflow_DELETE_NODES
				if err := updateWorkflowStateInDB(config.Name, clusterName, clusterView.ClusterWorkflows[clusterName], c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					return err
				}
				logger.Info().Msgf("Deleting nodes from cluster")
				cluster, err := deleteNodes(clusterView.CurrentClusters[clusterName], diff.ToDelete)
				if err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msgf("Failed to delete nodes")
					return err
				}

				clusterView.CurrentClusters[clusterName] = cluster
			}

			message := "Processing cluster"
			if diff.Stages() > 0 {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				message = fmt.Sprintf("Processing stage [%d/%d] for cluster", currentStage, stages)
			}
			logger.Info().Msgf(message)

			ctx := &BuilderContext{
				projectName:          config.Name,
				cluster:              clusterView.CurrentClusters[clusterName],
				desiredCluster:       clusterView.DesiredClusters[clusterName],
				loadbalancers:        clusterView.Loadbalancers[clusterName],
				desiredLoadbalancers: clusterView.DesiredLoadbalancers[clusterName],
				deletedLoadBalancers: clusterView.DeletedLoadbalancers[clusterName],
				Workflow:             clusterView.ClusterWorkflows[clusterName],
			}

			if ctx, err = buildCluster(ctx, c); err != nil {
				clusterView.SetWorkflowError(clusterName, err)
				logger.Err(err).Msg("Failed to build cluster")
				return err
			}

			// Propagate the changes made to the cluster back to the View.
			clusterView.UpdateFromBuild(ctx)

			if len(clusterView.DeletedLoadbalancers) > 0 {
				// perform the deletion of loadbalancers as this won't be handled by the buildCluster Workflow.
				// The BuildInfrastructure in terraformer only performs creation/update for Lbs.
				deleteCtx := &BuilderContext{
					projectName:   config.Name,
					loadbalancers: clusterView.DeletedLoadbalancers[clusterName],
					Workflow:      clusterView.ClusterWorkflows[clusterName],
				}

				if err := destroyCluster(deleteCtx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					logger.Err(err).Msg("Error while destroying cluster")
					return err
				}
			}

			// Workflow finished.
			clusterView.SetWorkflowDone(clusterName)
			if err := updateWorkflowStateInDB(config.Name, clusterName, ctx.Workflow, c); err != nil {
				clusterView.SetWorkflowError(clusterName, err)
				logger.Err(err).Msg("failed to save workflow for cluster")
				return err
			}

			logger.Info().Msg("Finished building cluster")
			return nil
		}); err != nil {
			logger.Err(err).Msg("Error encountered while processing config")
			// Even if the config fails to build merge the changes as it might be in an in-between state
			// in order to be able to delete it later.
			clusterView.MergeChanges(config)

			if err := saveConfigWithWorkflowError(config, c, clusterView); err != nil {
				log.Err(err).Msg("Failed to save error message")
			}
			return
		}

		// Propagate all the changes to the config.
		clusterView.MergeChanges(config)

		// Update the config and store it to the DB.
		logger.Debug().Msg("Saving the config")
		config.CurrentState = config.DesiredState
		if err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config}); err != nil {
			logger.Err(err).Msg("Error while saving the config")
			return
		}

		logger.Info().Msgf("Config finished building")
	}()

	return nil
}

// separateNodepools creates two slices of node names, one for master and one for worker nodes
func separateNodepools(clusterNodes map[string]int32, clusterInfo *pb.ClusterInfo) (master []string, worker []string) {
	for _, nodepool := range clusterInfo.NodePools {
		if count, ok := clusterNodes[nodepool.Name]; ok && count > 0 {
			names := getNodeNames(nodepool, int(count))
			if nodepool.IsControl {
				master = append(master, names...)
			} else {
				worker = append(worker, names...)
			}
		}
	}
	return master, worker
}

// getNodeNames returns slice of length count with names of the nodes from specified nodepool
// nodes chosen are from the last element in Nodes slice, up to the first one
func getNodeNames(nodepool *pb.NodePool, count int) (names []string) {
	for i := len(nodepool.Nodes) - 1; i >= len(nodepool.Nodes)-count; i-- {
		names = append(names, nodepool.Nodes[i].Name)
		log.Debug().Msgf("Choosing node %s for deletion", nodepool.Nodes[i].Name)
	}
	return names
}

func deleteNodes(cluster *pb.K8Scluster, nodes map[string]int32) (*pb.K8Scluster, error) {
	master, worker := separateNodepools(nodes, cluster.ClusterInfo)
	newCluster, err := callDeleteNodes(master, worker, cluster)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes for %s : %w", cluster.ClusterInfo.Name, err)
	}

	return newCluster, nil
}
