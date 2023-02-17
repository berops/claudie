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
		return destroyCluster(&BuilderContext{
			projectName:   config.Name,
			cluster:       cluster,
			loadbalancers: clusterView.Loadbalancers[cluster.ClusterInfo.Name],
		})
	}); err != nil {
		return err
	}

	return cbox.DeleteConfigFromDB(c, &pb.DeleteConfigRequest{Id: config.Id, Type: pb.IdType_HASH})
}

// destroy destroys any Loadbalancers or the cluster itself.
func destroy(projectName, clusterName string, clusterView *ClusterView) (bool, error) {
	deleteCtx := &BuilderContext{
		projectName: projectName,
	}

	if clusterView.Clusters[clusterName] != nil && clusterView.DesiredClusters[clusterName] == nil {
		deleteCtx.cluster = clusterView.Clusters[clusterName]
	}

	if len(clusterView.DeletedLoadbalancers[clusterName]) != 0 {
		deleteCtx.loadbalancers = clusterView.DeletedLoadbalancers[clusterName]
	}

	if deleteCtx.cluster != nil || len(deleteCtx.loadbalancers) > 0 {
		if err := destroyCluster(deleteCtx); err != nil {
			return false, err
		}

		// if there is no desired state for the cluster there is no more work to be done.
		if deleteCtx.cluster != nil {
			return true, nil
		}
	}

	return false, nil
}

// saveErrorMessage saves error message to config
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	if config.DesiredState != nil {
		// Update currentState preemptively, so we can use it for terraform destroy
		// id DesiredState is null, we are already in deletion process, thus CurrentState should stay as is when error occurs
		config.CurrentState = config.DesiredState
	}
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config in Builder: %w", err)
	}
	return nil
}
