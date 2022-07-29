package main

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type nodesToDelete struct {
	nodes map[string]*nodeCount // [nodepoolName]count which needs to be deleted
}

type nodeCount struct {
	Count uint32
}

type nodepoolKey struct {
	clusterName  string
	nodePoolName string
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
		if config == nil {
			return nil
		}
		//process config in goroutine to allow single Builder to work concurrently on multiple configs
		go func(config *pb.Config) {
			//tmpConfig is used in operation where config is adding && deleting the nodes
			//first, tmpConfig is applied which only adds nodes and only then the real config is applied, which will delete nodes
			var tmpConfig *pb.Config
			var toDelete map[string]*nodesToDelete
			//if any current state already exist, find difference
			if len(config.CurrentState.GetClusters()) > 0 {
				tmpConfig, toDelete = stateDifference(config)
			}
			//if tmpConfig is not nil, first apply it
			if tmpConfig != nil {
				log.Info().Msg("Processing a tmpConfig...")
				err := buildConfig(tmpConfig, c, true)
				if err != nil {
					log.Error().Err(err)
				}
				config.CurrentState = tmpConfig.DesiredState
			}
			if toDelete != nil {
				log.Info().Msg("Deleting nodes...")
				config, err = deleteNodes(config, toDelete)
				if err != nil {
					log.Error().Err(err)
				}
			}

			log.Info().Msgf("Processing config %s", config.Name)

			err = buildConfig(config, c, false)
			if err != nil {
				log.Error().Err(err)
			}
		}(config)
		return nil
	}
}

// stateDifference takes config to calculates difference between desired and current state to determine how many nodes  needs to be deleted and added.
func stateDifference(config *pb.Config) (*pb.Config, map[string]*nodesToDelete) {
	adding, deleting := false, false
	tmpConfig := proto.Clone(config).(*pb.Config)
	currentNodepoolMap := getNodepoolMap(tmpConfig.CurrentState.Clusters)
	var delCounts = make(map[string]*nodesToDelete)
	//iterate over clusters and find difference in nodepools
	for _, desiredClusterTmp := range tmpConfig.GetDesiredState().GetClusters() {
		delCounts[desiredClusterTmp.ClusterInfo.Name], adding, deleting = findNodepoolDifference(currentNodepoolMap, desiredClusterTmp)
	}

	//if any key left, it means that nodepool is defined in current state but not in the desired, i.e. whole nodepool should be deleted
	for key := range currentNodepoolMap {
		//find cluster in desired state
		desiredClusterTmp := utils.GetClusterByName(key.clusterName, tmpConfig.DesiredState.Clusters)
		if desiredClusterTmp != nil {
			//find cluster in current state
			currentCluster := utils.GetClusterByName(key.clusterName, tmpConfig.CurrentState.Clusters)
			if currentCluster != nil {
				//append nodepool to desired state, since tmpConfig only adds nodes
				desiredClusterTmp.ClusterInfo.NodePools = append(desiredClusterTmp.ClusterInfo.NodePools, utils.GetNodePoolByName(key.nodePoolName, currentCluster.ClusterInfo.GetNodePools()))
				//true since we will delete whole nodepool
				deleting = true
			}
		}
	}

	switch {
	case adding && deleting:
		return tmpConfig, delCounts
	case deleting:
		return nil, delCounts
	default:
		return nil, nil
	}
}

func getNodepoolMap(clusters []*pb.K8Scluster) map[nodepoolKey]nodeCount {
	nodepoolMap := make(map[nodepoolKey]nodeCount)
	for _, cluster := range clusters {
		for _, nodePool := range cluster.ClusterInfo.GetNodePools() {
			tmp := nodepoolKey{nodePoolName: nodePool.Name, clusterName: cluster.ClusterInfo.Name}
			nodepoolMap[tmp] = nodeCount{Count: nodePool.Count} // Since a nodepool as only one type of nodes, we'll need only one type of count
		}
	}
	return nodepoolMap
}

func findNodepoolDifference(currentNodepoolMap map[nodepoolKey]nodeCount, desiredClusterTmp *pb.K8Scluster) (result *nodesToDelete, adding bool, deleting bool) {
	nodepoolCounts := make(map[string]*nodeCount)
	//prepare the key
	nodepoolKey := nodepoolKey{clusterName: desiredClusterTmp.ClusterInfo.Name}
	//iterate over nodepools in cluster
	for _, nodePoolDesired := range desiredClusterTmp.ClusterInfo.GetNodePools() {
		var countToDelete nodeCount
		nodepoolKey.nodePoolName = nodePoolDesired.Name
		//if nodepool found in current nodepoolMap, check difference
		if nodePoolCurrent, ok := currentNodepoolMap[nodepoolKey]; ok {
			if nodePoolDesired.Count > nodePoolCurrent.Count {
				//if desired cluster has more nodes than in current nodepool
				adding = true
			} else if nodePoolDesired.Count < nodePoolCurrent.Count {
				//if desired cluster has less nodes than in current nodepool
				countToDelete.Count = nodePoolCurrent.Count - nodePoolDesired.Count
				//since we are working with tmp config, we do not delete nodes in this step, thus save the current node count
				nodePoolDesired.Count = nodePoolCurrent.Count
				deleting = true
			}
			//save nodepool and the count of nodes which will be deleted
			nodepoolCounts[nodePoolDesired.Name] = &countToDelete
			//delete nodepool from current nodepoolMap
			delete(currentNodepoolMap, nodepoolKey)
		}
	}
	result = &nodesToDelete{nodes: nodepoolCounts}
	return result, adding, deleting
}
