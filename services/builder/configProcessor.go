package main

import (
	"fmt"
	"sync"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
	"github.com/rs/zerolog/log"
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

		clusterView := NewClusterView(config)

		// if Desired state is null and current is not we delete the infra for the config.
		if config.DsChecksum == nil && config.CsChecksum != nil {
			if err := destroyConfig(config, clusterView, c); err != nil {
				// Save error to DB.
				log.Error().Msgf("Error while destroying config %s : %v", config.Name, err)
				if err := saveConfigWithWorkflowError(config, c, clusterView); err != nil {
					log.Error().Msgf("Failed to save error message for config %s:  %v", config.Name, err)
				}
			}
			return
		}

		if err := utils.ConcurrentExec(clusterView.AllClusters(), func(clusterName string) error {
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
				log.Info().Msgf("Processing stage [%d/%d] for cluster %s config %s", currentStage, stages, clusterName, config.Name)

				ctx := &BuilderContext{
					projectName:    config.Name,
					cluster:        clusterView.CurrentClusters[clusterName],
					desiredCluster: diff.IR,

					// ignore LBs for this step.
					loadbalancers:        nil,
					desiredLoadbalancers: nil,
					deletedLoadBalancers: nil,

					Workflow: clusterView.ClusterWorkflows[clusterName],
				}

				if ctx, err = buildCluster(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					log.Error().Msgf("Failed to build cluster %s project %s : %v", clusterName, config.Name, err)
					return err
				}
				log.Info().Msgf("First stage for cluster %s project %s finished building", clusterName, config.Name)

				// make the desired state of the temporary cluster the new current state.
				clusterView.CurrentClusters[clusterName] = ctx.desiredCluster
				clusterView.Loadbalancers[clusterName] = ctx.desiredLoadbalancers
			}

			if diff.ControlPlaneWithAPIEndpointReplace {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				log.Info().Msgf("Processing stage [%d/%d] for cluster %s config %s", currentStage, stages, clusterName, config.Name)

				ctx := &BuilderContext{
					projectName:    config.Name,
					cluster:        clusterView.CurrentClusters[clusterName],
					desiredCluster: clusterView.DesiredClusters[clusterName],
					Workflow:       clusterView.ClusterWorkflows[clusterName],
				}

				if err := callUpdateAPIEndpoint(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					log.Error().Msgf("Failed to build cluster %s project %s : %v", clusterName, config.Name, err)
					return err
				}

				clusterView.CurrentClusters[clusterName] = ctx.cluster
				clusterView.DesiredClusters[clusterName] = ctx.desiredCluster
				clusterView.DesiredLoadbalancers[clusterName] = ctx.desiredLoadbalancers

				ctx = &BuilderContext{
					projectName:          config.Name,
					desiredCluster:       clusterView.CurrentClusters[clusterName],
					desiredLoadbalancers: clusterView.Loadbalancers[clusterName],
					Workflow:             clusterView.ClusterWorkflows[clusterName],
				}

				if err := callKubeEleven(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					log.Error().Msgf("Failed to build cluster %s project %s : %v", clusterName, config.Name, err)
					return err
				}

				clusterView.CurrentClusters[clusterName] = ctx.desiredCluster
				clusterView.Loadbalancers[clusterName] = ctx.desiredLoadbalancers

				if err := callPatchClusterInfoConfigMap(ctx, c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					log.Error().Msgf("Failed to build cluster %s project %s : %v", clusterName, config.Name, err)
					return err
				}
			}

			if len(diff.ToDelete) > 0 {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				log.Info().Msgf("Processing stage [%d/%d] for cluster %s config %s", currentStage, stages, clusterName, config.Name)

				clusterView.ClusterWorkflows[clusterName].Stage = pb.Workflow_DELETE_NODES
				if err := updateWorkflowStateInDB(config.Name, clusterName, clusterView.ClusterWorkflows[clusterName], c); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					return err
				}
				log.Info().Msgf("Deleting nodes from cluster %s project %s", clusterName, config.Name)
				if clusterView.CurrentClusters[clusterName], err = deleteNodes(clusterView.CurrentClusters[clusterName], diff.ToDelete); err != nil {
					clusterView.SetWorkflowError(clusterName, err)
					log.Error().Msgf("Failed to delete nodes cluster %s project %s : %v", clusterName, config.Name, err)
					return err
				}
			}

			message := fmt.Sprintf("Processing cluster %s config %s", clusterName, config.Name)
			if diff.Stages() > 0 {
				currentStage++
				clusterView.ClusterWorkflows[clusterName].Description = fmt.Sprintf("Processing stage [%d/%d]", currentStage, stages)
				message = fmt.Sprintf("Processing stage [%d/%d] for cluster %s config %s", currentStage, stages, clusterName, config.Name)
			}
			log.Info().Msgf(message)

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
				log.Error().Msgf("Failed to build cluster %s project %s : %v", clusterName, config.Name, err)
				return err
			}

			// Propagate the changes made to the cluster back to the View.
			clusterView.UpdateFromBuild(ctx)

			// cleanup infra not present in current but not desired state.
			if err := destroy(config.Name, clusterName, clusterView, c); err != nil {
				clusterView.SetWorkflowError(clusterName, err)
				log.Error().Msgf("Error while destroying cluster %s project %s : %v", clusterName, config.Name, err)
				return err
			}

			clusterView.SetWorkflowDone(clusterName)

			if err := updateWorkflowStateInDB(config.Name, clusterName, ctx.Workflow, c); err != nil {
				clusterView.SetWorkflowError(clusterName, err)
				log.Error().Msgf("failed to save workflow for cluster %s project %s: %s", clusterName, config.Name, err)
				return err
			}

			log.Info().Msgf("Finished building cluster %s project %s", clusterName, config.Name)
			return nil
		}); err != nil {
			log.Error().Msgf("Error encountered while processing config %s : %v", config.Name, err)
			if err := saveConfigWithWorkflowError(config, c, clusterView); err != nil {
				log.Error().Msgf("Failed to save error message due to: %s", err)
			}
			return
		}

		// Propagate all the changes to the config.
		clusterView.MergeChanges(config)

		// Update the config and store it to the DB.
		log.Debug().Msgf("Saving the config %s", config.Name)
		config.CurrentState = config.DesiredState
		if err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config}); err != nil {
			log.Error().Msgf("error while saving the config %s: %s", config.GetName(), err)
			return
		}

		log.Info().Msgf("Config %s finished building", config.Name)
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
