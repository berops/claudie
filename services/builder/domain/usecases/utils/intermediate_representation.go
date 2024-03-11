package utils

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

	// IRLbs are the intermediate representations of LB clusters that should be passed through the workflow
	// before actually building the desired state of the LB clusters. If nil there is no in-between step.
	IRLbs []*pb.LBcluster

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

func lbClone(desiredLbs []*pb.LBcluster) []*pb.LBcluster {
	var result []*pb.LBcluster
	for _, lb := range desiredLbs {
		result = append(result, proto.Clone(lb).(*pb.LBcluster))
	}
	return result
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
		irLbs                       = lbClone(currentLbs)
		currentNodepoolCounts       = nodepoolsCounts(current)
		delCounts, adding, deleting = findNodepoolDifference(currentNodepoolCounts, ir, current)
		apiEndpointDeleted          = false
		apiLbTargetNodepoolDeleted  = false
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
				log.Debug().Msgf("Nodepool %s from cluster %s will be deleted", nodepoolName, current.ClusterInfo.Name)
				nodepool := utils.GetNodePoolByName(nodepoolName, current.ClusterInfo.GetNodePools())

				// check if the nodepool was an API-endpoint if yes we need to choose the next control nodepool as the endpoint.
				if _, err := utils.FindAPIEndpointNode([]*pb.NodePool{nodepool}); err == nil {
					apiEndpointDeleted = true
				}

				ir.ClusterInfo.NodePools = append(ir.ClusterInfo.NodePools, nodepool)

				// If There is an API endpoint LoadBalancer that has only one targetNodepool and we are removing
				// this target nodepool, we need to create an in-between step where we add a control nodepool
				// from the desired state, otherwise when in the stage deleting nodes, it will delete this nodepool
				// and the LoadBalancer will not be able to forward the request anywhere and the cluster state will be
				// corrupted.
				if utils.IsNodepoolOnlyTargetOfLbAPI(currentLbs, nodepool) {
					apiLbTargetNodepoolDeleted = true

					lbcluster := utils.FindLbAPIEndpointCluster(irLbs)

					// find other control nodepool that will not be deleted.
					var nextControlNodepool *pb.NodePool
					for _, cnp := range utils.FindControlNodepools(ir.GetClusterInfo().GetNodePools()) {
						if cnp.Name != nodepool.Name {
							nextControlNodepool = cnp
							break
						}
					}
					// No need to check if nextControlNodepool is nil. Validation of the inputmanifest
					// does not allow for the user to specify an empty list of control nodes
					//
					// Each nodepool after the scheduler stage has a hash appended to it.
					// to get the original nodepool name as defined in the input manifest
					// we need to strip the hash.
					nameWithoutHash := nextControlNodepool.Name[:len(nextControlNodepool.Name)-(utils.HashLength+1)] // +1 for '-'

					for _, role := range lbcluster.GetRoles() {
						if role.RoleType == pb.RoleType_ApiServer {
							role.TargetPools = append(role.TargetPools, nameWithoutHash)
							break
						}
					}
				}
			}
		}
	}

	result := &IntermediateRepresentation{
		ControlPlaneWithAPIEndpointReplace: apiEndpointDeleted,
	}

	// check if we're adding nodes and Api-server.
	addingLbApiEndpoint := current != nil && (!utils.HasLbAPIEndpoint(currentLbs) && utils.HasLbAPIEndpoint(desiredLbs))
	deletingLbApiEndpoint := current != nil && (utils.HasLbAPIEndpoint(currentLbs) && !utils.HasLbAPIEndpoint(desiredLbs))

	if apiLbTargetNodepoolDeleted || adding && deleting || (adding && addingLbApiEndpoint) || (adding && deletingLbApiEndpoint) {
		result.IR = ir
		result.IRLbs = irLbs
	}

	if deleting {
		result.ToDelete = delCounts
	}

	return result
}

// findNodepoolDifference calculates difference between desired nodepools and current nodepools.
func findNodepoolDifference(currentNodepoolCounts map[string]int32, desiredClusterTmp, currentClusterTmp *pb.K8Scluster) (result map[string]int32, adding, deleting bool) {
	nodepoolCountToDelete := make(map[string]int32)

	for _, nodePoolDesired := range desiredClusterTmp.GetClusterInfo().GetNodePools() {
		if nodePoolDesired.GetDynamicNodePool() != nil {
			currentCount, ok := currentNodepoolCounts[nodePoolDesired.Name]
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

			nodepoolCountToDelete[nodePoolDesired.Name] = countToDelete
			// keep track of which nodepools were deleted
			delete(currentNodepoolCounts, nodePoolDesired.Name)
		} else {
			currentCount, ok := currentNodepoolCounts[nodePoolDesired.Name]
			if !ok {
				// not in current state, adding.
				adding = true
				continue
			}
			if int32(len(nodePoolDesired.Nodes)) > currentCount {
				adding = true
			}

			var countToDelete int32
			if int32(len(nodePoolDesired.Nodes)) < currentCount {
				deleting = true
				countToDelete = currentCount - int32(len(nodePoolDesired.Nodes))
				// since we are working with tmp config, we do not delete nodes in this step, thus save the current nodes
				nodePoolDesired.Nodes = getStaticNodes(currentClusterTmp, nodePoolDesired)
			}

			nodepoolCountToDelete[nodePoolDesired.Name] = countToDelete
			// keep track of which nodepools were deleted
			delete(currentNodepoolCounts, nodePoolDesired.Name)
		}
	}
	return nodepoolCountToDelete, adding, deleting
}

// nodepoolsCounts returns a map for the counts in each nodepool for a cluster.
func nodepoolsCounts(cluster *pb.K8Scluster) map[string]int32 {
	counts := make(map[string]int32)
	for _, nodePool := range cluster.GetClusterInfo().GetNodePools() {
		if nodePool.GetDynamicNodePool() != nil {
			counts[nodePool.GetName()] = nodePool.GetDynamicNodePool().Count
		}
		if nodePool.GetStaticNodePool() != nil {
			counts[nodePool.GetName()] = int32(len(nodePool.Nodes))
		}
	}
	return counts
}

// getStaticNodes returns slice of nodes for the specified cluster from specified node pool.
func getStaticNodes(cluster *pb.K8Scluster, np *pb.NodePool) []*pb.Node {
	if np.GetStaticNodePool() == nil {
		return nil
	}
	for _, n := range cluster.ClusterInfo.NodePools {
		if n.GetStaticNodePool() != nil {
			if n.Name == np.Name {
				return np.Nodes
			}
		}
	}
	// Return desired nodes, and log error
	log.Warn().Msgf("No current static node pool found with name %s", np.Name)
	return np.Nodes
}
