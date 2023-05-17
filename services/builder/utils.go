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

// saveConfigWithWorkflowError saves config with workflow states
func saveConfigWithWorkflowError(config *pb.Config, c pb.ContextBoxServiceClient, clusterView *ClusterView) error {
	config.State = clusterView.ClusterWorkflows
	if config.DsChecksum != nil {
		config.CurrentState = config.DesiredState
	}
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
