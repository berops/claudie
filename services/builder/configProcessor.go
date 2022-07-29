package main

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type nodepoolsCounts struct {
	nodepools map[string]*nodeCount // [nodepoolName]count which needs to be deleted
}

type nodeCount struct {
	Count uint32
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
			var toDelete map[string]*nodepoolsCounts
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
func stateDifference(config *pb.Config) (*pb.Config, map[string]*nodepoolsCounts) {
	adding, deleting := false, false
	tmpConfig := proto.Clone(config).(*pb.Config)
	currentNodepoolMap := getNodepoolMap(tmpConfig.CurrentState.Clusters) //[clusterName][nodepoolName]Count
	var delCounts = make(map[string]*nodepoolsCounts)
	//iterate over clusters and find difference in nodepools
	for _, desiredClusterTmp := range tmpConfig.GetDesiredState().GetClusters() {
		delCounts[desiredClusterTmp.ClusterInfo.Name], adding, deleting = findNodepoolDifference(currentNodepoolMap, desiredClusterTmp)
	}

	//if any key left, it means that nodepool is defined in current state but not in the desired, i.e. whole nodepool should be deleted
	if len(currentNodepoolMap) > 0 {
		log.Info().Msgf("Detected deletion of a nodepools")
		deleting = true
		for clusterName, nodepoolsCount := range currentNodepoolMap {
			//merge maps together so delCounts holds all delete counts
			mergeDeleteCounts(delCounts[clusterName].nodepools, nodepoolsCount.nodepools)
			//since tmpConfig first adds nodes, the deleted nodes needs to be added into tmp Desired state
			desiredClusterTmp := utils.GetClusterByName(clusterName, tmpConfig.DesiredState.Clusters)
			if desiredClusterTmp != nil {
				//find cluster in current state
				currentCluster := utils.GetClusterByName(clusterName, tmpConfig.CurrentState.Clusters)
				if currentCluster != nil {
					//append nodepool to desired state, since tmpConfig only adds nodes
					for nodepoolName := range nodepoolsCount.nodepools {
						log.Info().Msgf("Nodepool %s from cluster %s will be deleted", nodepoolName, clusterName)
						desiredClusterTmp.ClusterInfo.NodePools = append(desiredClusterTmp.ClusterInfo.NodePools, utils.GetNodePoolByName(nodepoolName, currentCluster.ClusterInfo.GetNodePools()))
					}
				}
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

func getNodepoolMap(clusters []*pb.K8Scluster) map[string]*nodepoolsCounts {
	nodepoolMap := make(map[string]*nodepoolsCounts)
	for _, cluster := range clusters {
		npCount := &nodepoolsCounts{nodepools: make(map[string]*nodeCount)}
		for _, nodePool := range cluster.ClusterInfo.GetNodePools() {
			npCount.nodepools[nodePool.Name] = &nodeCount{Count: nodePool.Count}
		}
		nodepoolMap[cluster.ClusterInfo.Name] = npCount
	}
	return nodepoolMap
}

func findNodepoolDifference(currentNodepoolMap map[string]*nodepoolsCounts, desiredClusterTmp *pb.K8Scluster) (result *nodepoolsCounts, adding bool, deleting bool) {
	nodepoolCountToDelete := make(map[string]*nodeCount)
	//iterate over nodepools in desired cluster
	for _, nodePoolDesired := range desiredClusterTmp.ClusterInfo.GetNodePools() {
		//iterate over nodepools in current cluster
		nodepoolsCurrent := currentNodepoolMap[desiredClusterTmp.ClusterInfo.Name]
		for nodepoolCurrentName, nodePoolCurrentCount := range nodepoolsCurrent.nodepools {
			//if desired state contains nodepool from current, check counts
			if nodePoolDesired.Name == nodepoolCurrentName {
				var countToDelete nodeCount
				if nodePoolDesired.Count > nodePoolCurrentCount.Count { //if desired cluster has more nodes than in current nodepool
					adding = true
				} else if nodePoolDesired.Count < nodePoolCurrentCount.Count { //if desired cluster has less nodes than in current nodepool
					countToDelete.Count = nodePoolCurrentCount.Count - nodePoolDesired.Count
					//since we are working with tmp config, we do not delete nodes in this step, thus save the current node count
					nodePoolDesired.Count = nodePoolCurrentCount.Count
					deleting = true
				}
				nodepoolCountToDelete[nodePoolDesired.Name] = &countToDelete
				//delete nodepool from nodepool map, so we can keep track of which nodepools were deleted
				delete(nodepoolsCurrent.nodepools, nodePoolDesired.Name)
				//if cluster has no nodepools, delete the reference to cluster
				if len(nodepoolsCurrent.nodepools) == 0 {
					delete(currentNodepoolMap, desiredClusterTmp.ClusterInfo.Name)
				}
			}
		}
	}
	result = &nodepoolsCounts{nodepools: nodepoolCountToDelete}
	return result, adding, deleting
}

func mergeDeleteCounts(dst, src map[string]*nodeCount) map[string]*nodeCount {
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
