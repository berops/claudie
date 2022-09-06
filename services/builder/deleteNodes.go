package main

import (
	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

//deleteNodes function finds particular nodes for deletion and deletes them from the etcd and k8s clusters by calling Kuber
//function also changes config.Current state after the nodes are deleted, so current state reflects the real cluster state
//return config with new current state and nil if successful, nil and error  otherwise
func deleteNodes(config *pb.Config, toDelete map[string]*nodepoolsCounts) (*pb.Config, error) {
	var errGroup errgroup.Group
	for _, cluster := range config.CurrentState.Clusters {
		//get nodes to delete for this cluster
		clusterNodesToDelete, ok := toDelete[cluster.ClusterInfo.Name]
		if ok && clusterNodesToDelete == nil {
			func(clusterNodes *nodepoolsCounts, cluster *pb.K8Scluster) {
				//call DeleteNodes on kuber for this cluster
				errGroup.Go(func() error {
					//prepare data for Kuber
					master, worker := separateNodepools(clusterNodes, cluster.ClusterInfo)
					//send request to Kuber to delete nodes
					log.Info().Msgf("Deleting nodes for %s cluster", cluster.ClusterInfo.Name)
					newCluster, err := callDeleteNodes(master, worker, cluster)
					if err != nil {
						log.Error().Msgf("Error while deleting nodes for %s : %v", cluster.ClusterInfo.Name, err)
						return err
					}
					//Updation - Delete nodes from a current state Ips map
					cluster = newCluster
					return nil
				})
			}(clusterNodesToDelete, cluster)
		}
	}
	// wait until all cluster have deleted their nodes
	err := errGroup.Wait()
	if err != nil {
		return nil, err
	}
	return config, nil
}

//separateNodepools creates two slices of node names, one for master and one for worker nodes
func separateNodepools(clusterNodes *nodepoolsCounts, clusterInfo *pb.ClusterInfo) (master []string, worker []string) {
	for _, nodepool := range clusterInfo.NodePools {
		if count, ok := clusterNodes.nodepools[nodepool.Name]; ok && count != nil {
			if count.Count > 0 {
				names := getNodeNames(nodepool, int(count.Count))
				if nodepool.IsControl {
					master = append(master, names...)
				} else {
					worker = append(worker, names...)
				}
			}
		}
	}
	return master, worker
}

//getNodeNames returns slice of length count with names of the nodes from specified nodepool
func getNodeNames(nodepool *pb.NodePool, count int) (names []string) {
	for i := 0; i < count; i++ {
		names = append(names, nodepool.Nodes[i].Name)
	}
	return names
}
