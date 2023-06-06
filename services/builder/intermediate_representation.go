package main

import (
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

// IntermediateRepresentation represents the intermediate state
// for a pair of current, desired *pb.K8sClusters>
type IntermediateRepresentation struct {
	// IR is the intermediate representation that should be passed through the workflow
	// before actually building the desired state. If nil there is no in-between step.
	IR *pb.K8Scluster

	// ToDelete are the nodepools from which nodes needs to be deleted. This may be set
	// even if IR is nil.
	ToDelete map[string]int32

	// ControlPlaneWithAPIEndpointReplace if this is set it means that the nodepool in current state
	// which has an ApiEndpoint is deleted in desired state and needs to be updated
	// before executing the workflow for the desired state and before deleting the nodes
	// from the ToDelete.
	ControlPlaneWithAPIEndpointReplace bool
}

// Stages returns the number of individual stages. Useful for logging.
func (ir *IntermediateRepresentation) Stages() int {
	count := 0

	if ir.IR != nil {
		count++
	}

	if len(ir.ToDelete) > 0 {
		count++
	}

	if ir.ControlPlaneWithAPIEndpointReplace {
		count++
	}

	return count
}

// Diff takes the desired and current state to calculate difference between them to determine how many nodes  needs to be deleted and added.
func Diff(current, desired *pb.K8Scluster, currentLbs, desiredLbs []*pb.LBcluster) *IntermediateRepresentation {
	// we only care about the diff if both states are present.
	if current != nil && desired == nil || current == nil && desired != nil {
		return &IntermediateRepresentation{}
	}
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
	var (
		ir                          = proto.Clone(desired).(*pb.K8Scluster)
		currentNodepoolCounts       = nodepoolsCounts(current)
		delCounts, adding, deleting = findNodepoolDifference(currentNodepoolCounts, ir, current)
		apiEndpointDeleted          = false
	)

	// if any key left, it means that nodepool is defined in current state but not in the desired
	// i.e. whole nodepool should be deleted.
	if len(currentNodepoolCounts) > 0 {
		deleting = true

		// merge maps
		for k, v := range currentNodepoolCounts {
			delCounts[k] = v
		}

		// add the deleted nodes to the Desired state
		if current != nil && ir != nil {
			// append nodepool to desired state, since tmpConfig only adds nodes
			for nodepoolName := range currentNodepoolCounts {
				nodepool := utils.GetNodePoolByName(nodepoolName, current.ClusterInfo.GetNodePools())

				// check if the nodepool was an API-endpoint if yes we need to choose the next control nodepool as the endpoint.
				if _, err := utils.FindApiEndpointNode(nodepool); err == nil {
					apiEndpointDeleted = true
				}

				log.Debug().Msgf("Nodepool %s from cluster %s will be deleted", nodepoolName, current.ClusterInfo.Name)
				ir.ClusterInfo.NodePools = append(ir.ClusterInfo.NodePools, nodepool)
			}
		}
	}

	result := &IntermediateRepresentation{
		ControlPlaneWithAPIEndpointReplace: apiEndpointDeleted,
	}

	// check if we're adding nodes and Api-server.
	addingLbApiEndpoint := current != nil && (!utils.FindLbAPIEndpoint(currentLbs) && utils.FindLbAPIEndpoint(desiredLbs))

	if adding && deleting || (adding && addingLbApiEndpoint) {
		result.IR = ir
	}

	if deleting {
		result.ToDelete = delCounts
	}

	return result
}

func findNodepoolDifference(currentNodepoolCounts map[string]int32, desiredClusterTmp, currentClusterTmp *pb.K8Scluster) (result map[string]int32, adding, deleting bool) {
	nodepoolCountToDelete := make(map[string]int32)

	for _, nodePoolDesired := range desiredClusterTmp.GetClusterInfo().GetNodePools() {
		if nodePoolDesired.GetDynamicNodePool() != nil {
			currentCount, ok := currentNodepoolCounts[nodePoolDesired.GetDynamicNodePool().Name]
			if !ok {
				// not in current state, adding.
				adding = true
				continue
			}

			if nodePoolDesired.GetDynamicNodePool().Count > currentCount {
				adding = true
			}

			var countToDelete int32
			if nodePoolDesired.GetDynamicNodePool().Count < currentCount {
				deleting = true
				countToDelete = currentCount - nodePoolDesired.GetDynamicNodePool().Count
				// since we are working with tmp config, we do not delete nodes in this step, thus save the current node count
				nodePoolDesired.GetDynamicNodePool().Count = currentCount
			}

			nodepoolCountToDelete[nodePoolDesired.GetDynamicNodePool().Name] = countToDelete
			// keep track of which nodepools were deleted
			delete(currentNodepoolCounts, nodePoolDesired.GetDynamicNodePool().Name)
		} else {
			currentCount, ok := currentNodepoolCounts[nodePoolDesired.GetStaticNodePool().Name]
			if !ok {
				// not in current state, adding.
				adding = true
				continue
			}
			if int32(len(nodePoolDesired.GetStaticNodePool().Nodes)) > currentCount {
				adding = true
			}

			var countToDelete int32
			if int32(len(nodePoolDesired.GetStaticNodePool().Nodes)) < currentCount {
				deleting = true
				countToDelete = currentCount - int32(len(nodePoolDesired.GetStaticNodePool().Nodes))
				// since we are working with tmp config, we do not delete nodes in this step, thus save the current nodes
				nodePoolDesired.GetStaticNodePool().Nodes = getStaticNodes(currentClusterTmp, nodePoolDesired)
			}

			nodepoolCountToDelete[nodePoolDesired.GetStaticNodePool().Name] = countToDelete
			// keep track of which nodepools were deleted
			delete(currentNodepoolCounts, nodePoolDesired.GetStaticNodePool().Name)
		}
	}
	return nodepoolCountToDelete, adding, deleting
}

// nodepoolsCounts returns a map for the counts in each nodepool for a cluster.
func nodepoolsCounts(cluster *pb.K8Scluster) map[string]int32 {
	counts := make(map[string]int32)
	for _, nodePool := range cluster.GetClusterInfo().GetNodePools() {
		if nodePool.GetDynamicNodePool() != nil {
			counts[nodePool.GetDynamicNodePool().GetName()] = nodePool.GetDynamicNodePool().Count
		}
		if nodePool.GetStaticNodePool() != nil {
			counts[nodePool.GetStaticNodePool().GetName()] = int32(len(nodePool.GetStaticNodePool().Nodes))
		}
	}
	return counts
}

func getStaticNodes(cluster *pb.K8Scluster, np *pb.NodePool) []*pb.Node {
	if np.GetStaticNodePool() == nil {
		return nil
	}
	for _, n := range cluster.ClusterInfo.NodePools {
		if n.GetStaticNodePool() != nil {
			if n.GetStaticNodePool().Name == np.GetStaticNodePool().Name {
				return np.GetStaticNodePool().Nodes
			}
		}
	}
	// Return desired nodes, and log error
	log.Warn().Msgf("No current static node pool found with name %s", np.GetStaticNodePool().Name)
	return np.GetStaticNodePool().Nodes
}
