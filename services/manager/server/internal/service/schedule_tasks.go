package service

import (
	"fmt"
	"time"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func scheduleTasks(scheduled *store.Config) error {
	scheduledGRPC, err := store.ConvertToGRPC(scheduled)
	if err != nil {
		return fmt.Errorf("failed to convert database representation to GRPC for %q: %w", scheduled.Name, err)
	}

	for cluster, state := range scheduledGRPC.Clusters {
		var events []*spec.TaskEvent
		switch {
		// create
		case state.Current == nil:
			events = append(events, &spec.TaskEvent{
				Id:        uuid.New().String(),
				Timestamp: timestamppb.New(time.Now().UTC()),
				Event:     spec.Event_CREATE,
				Task: &spec.Task{
					CreateState: &spec.CreateState{
						K8S: state.Desired.GetK8S(),
						Lbs: state.Desired.GetLoadBalancers().GetClusters(),
					},
				},
			})
		// delete
		case state.Desired == nil:
			events = append(events, &spec.TaskEvent{
				Id:        uuid.New().String(),
				Timestamp: timestamppb.New(time.Now().UTC()),
				Event:     spec.Event_DELETE,
				Task: &spec.Task{
					DeleteState: &spec.DeleteState{
						K8S: state.Current.GetK8S(),
						Lbs: state.Current.GetLoadBalancers().GetClusters(),
					},
				},
			})
		// update
		default:
			//panic("implement me.")
		}

		if e := state.Events; e == nil {
			state.Events = &spec.Events{Events: events, Ttl: 0}
		} else {
			if len(e.Events) != 0 {
				return fmt.Errorf("failed to schedule tasks for cluster %q config %q. Cannot schedule tasks for a config which previous scheduled tasks have not been finished", cluster, scheduled.Name)
			}
			state.Events.Events = append(state.Events.Events, events...)
		}
	}

	db, err := store.ConvertFromGRPC(scheduledGRPC)
	if err != nil {
		return fmt.Errorf("failed to convert GRPC representation to database for %q: %w", scheduled.Name, err)
	}

	*scheduled = *db
	return nil
}

// Diff takes the desired and current state to calculate difference between them to determine the difference and returns
// a number of tasks to be performed in specific order.
func Diff(current, desired *spec.K8Scluster, currentLbs, desiredLbs []*spec.LBcluster) []*spec.TaskEvent {
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
		ir                          = proto.Clone(desired).(*spec.K8Scluster)
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
				if _, err := utils.FindAPIEndpointNode([]*spec.NodePool{nodepool}); err == nil {
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
					var nextControlNodepool *spec.NodePool
					for _, cnp := range utils.FindControlNodepools(ir.GetClusterInfo().GetNodePools()) {
						if cnp.Name != nodepool.Name {
							nextControlNodepool = cnp
							break
						}
					}
					// No need to check if nextControlNodepool is nil. Validation of the inputmanifest
					// does not allow for the user to specify an empty list of control nodes
					nameWithoutHash := nextControlNodepool.Name
					// Each dynamic nodepool after the scheduler stage has a hash appended to it.
					// to get the original nodepool name as defined in the input manifest
					// we need to strip the hash.
					if nextControlNodepool.GetDynamicNodePool() != nil {
						nameWithoutHash = nextControlNodepool.Name[:len(nextControlNodepool.Name)-(utils.HashLength+1)] // +1 for '-'
					}

					for _, role := range lbcluster.GetRoles() {
						if role.RoleType == spec.RoleType_ApiServer {
							role.TargetPools = append(role.TargetPools, nameWithoutHash)
							break
						}
					}
				}
			}
		}
	}

	var events []*spec.TaskEvent

	// check if we're adding nodes and Api-server.
	addingLbApiEndpoint := current != nil && (!utils.HasLbAPIEndpoint(currentLbs) && utils.HasLbAPIEndpoint(desiredLbs))
	deletingLbApiEndpoint := current != nil && (utils.HasLbAPIEndpoint(currentLbs) && !utils.HasLbAPIEndpoint(desiredLbs))

	if apiLbTargetNodepoolDeleted || adding && deleting || (adding && addingLbApiEndpoint) || (adding && deletingLbApiEndpoint) {
		events = append(events, &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: ir,
					Lbs: irLbs,
				},
			},
		})
	}

	if apiEndpointDeleted {
		events = append(events, &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					ControlPlaneWithAPIEndpointReplace: addingLbApiEndpoint,
				},
			},
		})
	}

	if deleting {
		events = append(events, &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				DeleteState: &spec.DeleteState{
					Nodepools: delCounts,
				},
			},
		})
	}

	return events
}

// findNodepoolDifference calculates difference between desired nodepools and current nodepools.
func findNodepoolDifference(currentNodepoolCounts map[string]int32, desiredClusterTmp, currentClusterTmp *spec.K8Scluster) (result map[string]int32, adding, deleting bool) {
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
func nodepoolsCounts(cluster *spec.K8Scluster) map[string]int32 {
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
func getStaticNodes(cluster *spec.K8Scluster, np *spec.NodePool) []*spec.Node {
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

func lbClone(desiredLbs []*spec.LBcluster) []*spec.LBcluster {
	var result []*spec.LBcluster
	for _, lb := range desiredLbs {
		result = append(result, proto.Clone(lb).(*spec.LBcluster))
	}
	return result
}
