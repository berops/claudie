package main

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
)

// destroyConfig destroys all the current state of the config.
func destroyConfig(config *pb.Config, clusterView *utils.ClusterView, c pb.ContextBoxServiceClient) error {
	if err := utils.ConcurrentExec(config.CurrentState.Clusters, func(_ int, cluster *pb.K8Scluster) error {
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
func saveConfigWithWorkflowError(config *pb.Config, c pb.ContextBoxServiceClient, clusterView *utils.ClusterView) error {
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

// updateNodePoolInfo updates the nodepool metadata and node private IPs between stages of the cluster build.
func updateNodePoolInfo(src []*pb.NodePool, dst []*pb.NodePool) {
src:
	for _, npSrc := range src {
		for _, npDst := range dst {
			if npSrc.Name == npDst.Name {
				srcNodes := getNodeMap(npSrc.Nodes)
				dstNodes := getNodeMap(npDst.Nodes)
				for dstName, dstNode := range dstNodes {
					if srcNode, ok := srcNodes[dstName]; ok {
						dstNode.Private = srcNode.Private
					}
				}
				if npSrc.GetDynamicNodePool() != nil && npDst.GetDynamicNodePool() != nil {
					npDst.GetDynamicNodePool().Metadata = npSrc.GetDynamicNodePool().Metadata
				}
				continue src
			}
		}
	}
}

// getNodeMap returns a map of nodes, where each key is node name.
func getNodeMap(nodes []*pb.Node) map[string]*pb.Node {
	m := make(map[string]*pb.Node, len(nodes))
	for _, node := range nodes {
		m[node.Name] = node
	}
	return m
}
