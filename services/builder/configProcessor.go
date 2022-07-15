package main

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type countsToDelete struct {
	Count uint32
}

type nodesToDelete struct {
	nodes map[string]*countsToDelete // [provider]nodes
}

// configProcessor takes in cbox client to receive the configs.
// It then calculated the changes that needs to be done in order
// to see if nodes needs to be deleted or added or both. It also
// create a tempConfig to divide the addition and deletion of nodes
// into two steps and call respective functions.
func configProcessor(c pb.ContextBoxServiceClient) func() error {
	return func() error {
		res, err := cbox.GetConfigBuilder(c) // Get a new config
		if err != nil {
			return fmt.Errorf("error while getting config from the Builder: %v", err)
		}

		config := res.GetConfig()
		if config != nil {
			var tmpConfig *pb.Config
			var deleting bool
			var toDelete = make(map[string]*nodesToDelete)
			if len(config.CurrentState.GetClusters()) > 0 {
				tmpConfig, deleting, toDelete = diff(config)
			}
			if tmpConfig != nil {
				log.Info().Msg("Processing a tmpConfig...")
				err := buildConfig(tmpConfig, c, true)
				if err != nil {
					return err
				}
				config.CurrentState = tmpConfig.DesiredState
			}
			if deleting {
				log.Info().Msg("Deleting nodes...")
				config, err = deleteNodes(config, toDelete)
				if err != nil {
					return err
				}
			}

			log.Info().Msgf("Processing config %s", config.Name)
			go func() {
				err := buildConfig(config, c, false)
				if err != nil {
					log.Error().Err(err)
				}
			}()
		}
		return nil
	}
}

// diff takes config to calculate which nodes needs to be deleted and added.
func diff(config *pb.Config) (*pb.Config, bool, map[string]*nodesToDelete) {
	adding, deleting := false, false
	tmpConfig := proto.Clone(config).(*pb.Config)

	type nodeCount struct {
		Count uint32
	}

	type nodepoolKey struct {
		clusterName  string
		nodePoolName string
	}

	var delCounts = make(map[string]*nodesToDelete)

	var nodepoolMap = make(map[nodepoolKey]nodeCount)
	for _, cluster := range tmpConfig.GetCurrentState().GetClusters() {
		for _, nodePool := range cluster.ClusterInfo.GetNodePools() {
			tmp := nodepoolKey{nodePoolName: nodePool.Name, clusterName: cluster.ClusterInfo.Name}
			nodepoolMap[tmp] = nodeCount{Count: nodePool.Count} // Since a nodepool as only one type of nodes, we'll need only one type of count
		}
	}

	for _, cluster := range tmpConfig.GetDesiredState().GetClusters() {
		tmp := make(map[string]*countsToDelete)
		for _, nodePool := range cluster.ClusterInfo.GetNodePools() {
			var nodesProvider countsToDelete
			key := nodepoolKey{nodePoolName: nodePool.Name, clusterName: cluster.ClusterInfo.Name}

			if _, ok := nodepoolMap[key]; ok {
				tmpNodePool := utils.GetNodePoolByName(nodePool.Name, utils.GetClusterByName(cluster.ClusterInfo.Name, tmpConfig.GetDesiredState().GetClusters()).ClusterInfo.GetNodePools())
				if nodePool.Count > nodepoolMap[key].Count {
					tmpNodePool.Count = nodePool.Count
					adding = true
				} else if nodePool.Count < nodepoolMap[key].Count {
					nodesProvider.Count = nodepoolMap[key].Count - nodePool.Count
					tmpNodePool.Count = nodepoolMap[key].Count
					deleting = true
				}

				tmp[nodePool.Name] = &nodesProvider
				delete(nodepoolMap, key)
			}
		}
		delCounts[cluster.ClusterInfo.Name] = &nodesToDelete{
			nodes: tmp,
		}
	}

	if len(nodepoolMap) > 0 {
		for key := range nodepoolMap {
			cluster := utils.GetClusterByName(key.clusterName, tmpConfig.DesiredState.Clusters)
			if cluster != nil {
				currentCluster := utils.GetClusterByName(key.clusterName, tmpConfig.CurrentState.Clusters)
				log.Info().Interface("currentCluster", currentCluster)
				cluster.ClusterInfo.NodePools = append(cluster.ClusterInfo.NodePools, utils.GetNodePoolByName(key.nodePoolName, currentCluster.ClusterInfo.GetNodePools()))
				deleting = true
			}
		}
	}

	switch {
	case adding && deleting:
		return tmpConfig, deleting, delCounts
	case deleting:
		return nil, deleting, delCounts
	default:
		return nil, deleting, nil
	}
}
