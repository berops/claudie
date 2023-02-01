package main

/*
How operations with the nodes work:

We can have three cases of a operation within the input manifest

- just addition of a nodes
  - the config is processed right away

- just deletion of a nodes
  - firstly, the nodes are deleted from the cluster (via kubectl)
  - secondly, the config is  processed which will delete the nodes from infra

- addition AND deletion of the nodes
  - firstly the tmpConfig is applied, which will only add nodes into the cluster
  - secondly, the nodes are deleted from the cluster (via kubectl)
  - lastly, the config is processed, which will delete the nodes from infra
*/

import (
	"fmt"
	"sync"

	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	cbox "github.com/Berops/claudie/services/context-box/client"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type nodepoolsCounts struct {
	nodepools map[string]*nodeCount // [nodepoolName]count which needs to be deleted
}

type nodeCount struct {
	Count uint32
}

// configProcessor will fetch new configs from the context-box service. Each received config will be processed in
// a separate go-routine. If a sync.WaitGroup is supplied it will call the Add(1) and then the Done() method on it
// after the go-routine finishes the work, if nil it will be ignored.
func configProcessor(c pb.ContextBoxServiceClient, wg *sync.WaitGroup) error {
	res, err := cbox.GetConfigBuilder(c) // Get a new config
	if err != nil {
		return fmt.Errorf("error while getting config from the Context-box: %w", err)
	}

	builderCtx := &BuilderContext{Config: res.GetConfig()}
	if builderCtx.Config == nil {
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

		// check if Desired state is null and if so we want to delete the existing config
		if builderCtx.Config.DsChecksum == nil && builderCtx.Config.CsChecksum != nil {
			if err := destroyConfigAndDeleteDoc(builderCtx.Config, c); err != nil {
				log.Error().Msgf("failed to delete the config %s : %v", builderCtx.Config.Name, err)
			}
			return
		}

		if builderCtx.DeletedConfig = getDeletedClusterConfig(builderCtx.Config); builderCtx.DeletedConfig != nil {
			if err := destroyConfig(builderCtx.DeletedConfig, c); err != nil {
				log.Error().Msgf("failed to delete clusters from config %s : %v", builderCtx.Config.Name, err)
				return
			}
		}

		//tmpConfig is used in operation where config is adding && deleting the nodes
		//first, tmpConfig is applied which only adds nodes and only then the real config is applied, which will delete nodes
		tmpCtx := &BuilderContext{DeletedConfig: builderCtx.DeletedConfig}
		var toDelete map[string]*nodepoolsCounts //[clusterName]nodepoolsCount

		//if any current state already exist, find difference
		if len(builderCtx.Config.CurrentState.GetClusters()) > 0 {
			tmpCtx.Config, toDelete = stateDifference(builderCtx.Config)
		}

		if tmpCtx.Config != nil {
			log.Info().Msgf("Processing stage [1/2] for config %s", tmpCtx.Config.Name)
			if err := buildConfig(tmpCtx, c, true); err != nil {
				log.Error().Msgf("error while processing config %s : %v", tmpCtx.Config.Name, err)
				return
			}
			log.Info().Msgf("First stage of config %s finished building", tmpCtx.Config.Name)
			builderCtx.Config.CurrentState = tmpCtx.Config.DesiredState
		}

		if toDelete != nil {
			name := builderCtx.Config.Name
			log.Info().Msgf("Deleting nodes for config %s", name)
			builderCtx.Config, err = deleteNodes(builderCtx.Config, toDelete)
			if err != nil {
				log.Error().Msgf("error while deleting nodes for config %s : %v", name, err)
				return
			}
		}

		message := fmt.Sprintf("Processing config %s", builderCtx.Config.Name)
		if tmpCtx.Config != nil {
			message = fmt.Sprintf("Processing stage [2/2] for config %s", builderCtx.Config.Name)
		}
		log.Info().Msgf(message)

		if err = buildConfig(builderCtx, c, false); err != nil {
			log.Error().Msgf("error while processing config %s : %v", builderCtx.Config.Name, err)
			return
		}
		log.Info().Msgf("Config %s finished building", builderCtx.Config.Name)
	}()

	return nil
}

// stateDifference takes config to calculates difference between desired and current state to determine how many nodes  needs to be deleted and added.
func stateDifference(config *pb.Config) (*pb.Config, map[string]*nodepoolsCounts) {
	adding, deleting := false, false
	tmpConfig := proto.Clone(config).(*pb.Config)
	currentNodepoolMap := getNodepoolMap(tmpConfig.CurrentState.Clusters) //[clusterName][nodepoolName]Count
	var delCounts = make(map[string]*nodepoolsCounts)                     //[clusterName]nodepoolCount
	//iterate over clusters and find difference in nodepools
	for _, desiredClusterTmp := range tmpConfig.GetDesiredState().GetClusters() {
		npCounts, add, del := findNodepoolDifference(currentNodepoolMap, desiredClusterTmp)
		delCounts[desiredClusterTmp.ClusterInfo.Name] = npCounts
		if add {
			adding = true
		}
		if del {
			deleting = true
		}
	}

	//if any key left, it means that nodepool is defined in current state but not in the desired, i.e. whole nodepool should be deleted
	if len(currentNodepoolMap) > 0 {
		log.Debug().Msgf("Detected deletion of a nodepools")
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
						log.Debug().Msgf("Nodepool %s from cluster %s will be deleted", nodepoolName, clusterName)
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

// getNodepoolMap returns a map in a form of map[ClusterName]nodecount{map[NodepoolName]count}
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

// findNodepoolDifference will find any difference in nodepool between desired state and current
// this function should be used only with tmpConfig, since it will augment the desired state in a way, that will not delete the nodes
// returns count of nodes to delete in form of map[NodepoolName]counts,and booleans about deletion and addition of any nodes
func findNodepoolDifference(currentNodepoolMap map[string]*nodepoolsCounts, desiredClusterTmp *pb.K8Scluster) (result *nodepoolsCounts, adding bool, deleting bool) {
	nodepoolCountToDelete := make(map[string]*nodeCount)
	//iterate over nodepools in desired cluster
	for _, nodePoolDesired := range desiredClusterTmp.ClusterInfo.GetNodePools() {
		//iterate over nodepools in current cluster
		if nodepoolsCurrent, ok := currentNodepoolMap[desiredClusterTmp.ClusterInfo.Name]; ok {
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
		} else {
			//adding a new nodepool, since not found in current state
			adding = true
		}
	}
	result = &nodepoolsCounts{nodepools: nodepoolCountToDelete}
	return result, adding, deleting
}

// mergeDeleteCounts function will merge two maps which hold info about deletion of the nodes into one
// return map of the nodes for deletion in for of map[ClusterName]nodecount{map[NodepoolName]count}
func mergeDeleteCounts(dst, src map[string]*nodeCount) map[string]*nodeCount {
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// getDeletedClusterConfig function queries for cluster those needs ro be deleted from current state.
// It also updated the config object to remove the clusters to be deleted from current state. Thus
// the function has SIDE EFFECTS and should be used carefully.
// returns *pb.Config which contains clusters (both k8s and lb) that needs to be deleted.
func getDeletedClusterConfig(config *pb.Config) *pb.Config {
	if config.CurrentState.Name == "" {
		//if first run of config, skip
		return nil
	}
	configToDelete := proto.Clone(config).(*pb.Config)
	var k8sClustersToDelete, remainingCsK8sClusters []*pb.K8Scluster
	var LbClustersToDelete, remainingCsLbClusters []*pb.LBcluster

OuterK8s:
	for _, csCluster := range config.CurrentState.Clusters {
		for _, dsCluster := range config.DesiredState.Clusters {
			if isEqual(dsCluster.ClusterInfo, csCluster.ClusterInfo) {
				remainingCsK8sClusters = append(remainingCsK8sClusters, proto.Clone(csCluster).(*pb.K8Scluster))
				continue OuterK8s
			}
		}
		k8sClustersToDelete = append(k8sClustersToDelete, proto.Clone(csCluster).(*pb.K8Scluster))
	}
OuterLb:
	for _, csLbCluster := range config.CurrentState.LoadBalancerClusters {
		for _, dsLbCluster := range config.DesiredState.LoadBalancerClusters {
			if isEqual(dsLbCluster.ClusterInfo, csLbCluster.ClusterInfo) {
				remainingCsLbClusters = append(remainingCsLbClusters, proto.Clone(csLbCluster).(*pb.LBcluster))
				continue OuterLb
			}
		}
		LbClustersToDelete = append(LbClustersToDelete, proto.Clone(csLbCluster).(*pb.LBcluster))
	}
	configToDelete.CurrentState.Clusters = k8sClustersToDelete
	configToDelete.CurrentState.LoadBalancerClusters = LbClustersToDelete

	// update the passed config's currentState to remove the clusters which will be deleted
	config.CurrentState.Clusters = remainingCsK8sClusters
	config.CurrentState.LoadBalancerClusters = remainingCsLbClusters
	return configToDelete
}

// isEqual function checks if the two cluster from desiredState and Current state are same by comparing
// names and hashes
// return boolean value, True if the match otherwise False
func isEqual(dsClusterInfo, csClusterInfo *pb.ClusterInfo) bool {
	return dsClusterInfo.Name == csClusterInfo.Name && dsClusterInfo.Hash == csClusterInfo.Hash
}
