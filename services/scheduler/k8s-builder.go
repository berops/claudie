package main

import (
	"fmt"
	"strings"

	"github.com/Berops/platform/internal/utils"
	"github.com/Berops/platform/pkg/manifest"
	"github.com/Berops/platform/proto/pb"
)

//createK8sCluster reads manifest state and create kubernetes clusters based on it
//returns slice of *pb.K8Scluster if successful, nil otherwise
func createK8sCluster(manifestState *manifest.Manifest) ([]*pb.K8Scluster, error) {
	var clusters []*pb.K8Scluster
	//loop through clusters from manifest
	for _, cluster := range manifestState.Kubernetes.Clusters {
		//generate variables
		newCluster := &pb.K8Scluster{
			ClusterInfo: &pb.ClusterInfo{
				Name: strings.ToLower(cluster.Name),
				Hash: utils.CreateHash(utils.HashLength),
			},
			Kubernetes: cluster.Version,
			Network:    cluster.Network,
		}
		// createNodepools
		controlNodePools, err := manifestState.CreateNodepools(cluster.Pools.Control, true)
		if err != nil {
			return nil, fmt.Errorf("error while creating control nodepool for %s : %v", cluster.Name, err)
		}
		computeNodePools, err := manifestState.CreateNodepools(cluster.Pools.Compute, false)
		if err != nil {
			return nil, fmt.Errorf("error while creating compute nodepool for %s : %v", cluster.Name, err)
		}
		newCluster.ClusterInfo.NodePools = append(controlNodePools, computeNodePools...)
		clusters = append(clusters, newCluster)
	}
	return clusters, nil
}

//updateK8sClusters updates the desired state of the kubernetes clusters based on the current state
//returns error if failed, nil otherwise
func updateK8sClusters(newConfig *pb.Config) error {
clusterDesired:
	for _, clusterDesired := range newConfig.DesiredState.Clusters {
		for _, clusterCurrent := range newConfig.CurrentState.Clusters {
			// found current cluster with matching name
			if clusterDesired.ClusterInfo.Name == clusterCurrent.ClusterInfo.Name {
				updateClusterInfo(clusterDesired.ClusterInfo, clusterCurrent.ClusterInfo)
				if clusterCurrent.Kubeconfig != "" {
					clusterDesired.Kubeconfig = clusterCurrent.Kubeconfig
				}
				//skip the checks bellow
				continue clusterDesired
			}
		}
		// no current cluster found with matching name, create keys
		if clusterDesired.ClusterInfo.PublicKey == "" {
			err := createKeys(clusterDesired.ClusterInfo)
			if err != nil {
				return fmt.Errorf("error encountered while creating desired state for %s : %v", clusterDesired.ClusterInfo.Name, err)
			}
		}
	}
	return nil
}
