package main

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
)

// destroyConfig destroys all the current state of the config.
func destroyConfig(config *pb.Config, clusterView *ClusterView, c pb.ContextBoxServiceClient) error {
	if err := utils.ConcurrentExec(config.CurrentState.Clusters, func(cluster *pb.K8Scluster) error {
		err := destroyCluster(&BuilderContext{
			projectName:   config.Name,
			cluster:       cluster,
			loadbalancers: clusterView.Loadbalancers[cluster.ClusterInfo.Name],
			Workflow:      clusterView.ClusterWorkflows[cluster.ClusterInfo.Name],
		}, c)

		if err != nil {
			clusterView.SetWorkflowError(cluster.ClusterInfo.Name, err)
			return err
		}

		clusterView.SetWorkflowDone(cluster.ClusterInfo.Name)
		return nil
	}); err != nil {
		return err
	}

	return cbox.DeleteConfigFromDB(c, &pb.DeleteConfigRequest{Id: config.Id, Type: pb.IdType_HASH})
}

// destroy destroys any Loadbalancers or the cluster itself.
func destroy(projectName, clusterName string, clusterView *ClusterView, c pb.ContextBoxServiceClient) (bool, error) {
	deleteCtx := &BuilderContext{
		projectName: projectName,
		Workflow:    clusterView.ClusterWorkflows[clusterName],
	}

	if clusterView.CurrentClusters[clusterName] != nil && clusterView.DesiredClusters[clusterName] == nil {
		deleteCtx.cluster = clusterView.CurrentClusters[clusterName]
	}

	if len(clusterView.DeletedLoadbalancers[clusterName]) != 0 {
		deleteCtx.loadbalancers = clusterView.DeletedLoadbalancers[clusterName]
	}

	if deleteCtx.cluster != nil || len(deleteCtx.loadbalancers) > 0 {
		if err := destroyCluster(deleteCtx, c); err != nil {
			return false, err
		}

		// if there is no desired state for the cluster there is no more work to be done.
		if deleteCtx.cluster != nil {
			return true, nil
		}
	}

	return false, nil
}

// saveConfigWithWorkflowError saves config with workflow states
func saveConfigWithWorkflowError(config *pb.Config, c pb.ContextBoxServiceClient, clusterView *ClusterView) error {
	// Update Current state only when config is not marked for deletion
	// NOTE: reworked in PR https://github.com/berops/claudie/pull/728
	if config.DsChecksum != nil {
		config.CurrentState = config.DesiredState
	}

	config.State = clusterView.ClusterWorkflows

	return cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
}

func updateWorkflowStateInDB(configName, clusterName string, wf *pb.Workflow, c pb.ContextBoxServiceClient) error {
	if configName == "" {
		return fmt.Errorf("config name must not be empty when updating workflow state")
	}

	if clusterName == "" {
		return fmt.Errorf("cluster name must not be empty when updating workflow state")
	}

	return cbox.SaveWorkflowState(c, &pb.SaveWorkflowStateRequest{
		ConfigName:  configName,
		ClusterName: clusterName,
		Workflow:    wf,
	})
}
