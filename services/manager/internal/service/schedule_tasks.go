package service

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func scheduleTasks(scheduled *store.Config) (bool, error) {
	scheduledGRPC, err := store.ConvertToGRPC(scheduled)
	if err != nil {
		return false, fmt.Errorf("failed to convert database representation to GRPC for %q: %w", scheduled.Name, err)
	}

	var reschedule bool

	for cluster, state := range scheduledGRPC.Clusters {
		var events []*spec.TaskEvent
		switch {
		case state.Current == nil && state.Desired == nil:
			// nothing to do (desired state was not build).
		// create
		case state.Current == nil && state.Desired != nil:
			events = append(events, &spec.TaskEvent{
				Id:          uuid.New().String(),
				Timestamp:   timestamppb.New(time.Now().UTC()),
				Event:       spec.Event_CREATE,
				Description: "creating cluster",
				Task: &spec.Task{
					CreateState: &spec.CreateState{
						K8S: state.Desired.GetK8S(),
						Lbs: state.Desired.GetLoadBalancers(),
					},
				},
			})
		// delete
		case state.Desired == nil && state.Current != nil:
			events = append(events, &spec.TaskEvent{
				Id:          uuid.New().String(),
				Timestamp:   timestamppb.New(time.Now().UTC()),
				Event:       spec.Event_DELETE,
				Description: "deleting cluster",
				Task: &spec.Task{
					DeleteState: &spec.DeleteState{
						K8S: state.Current.GetK8S(),
						Lbs: state.Current.GetLoadBalancers(),
					},
				},
			})
		// update
		default:
			if state.State.Status == spec.Workflow_ERROR {
				if len(state.Events.Events) != 0 && state.Events.Events[0].OnError != nil {
					reschedule = true

					switch e := state.Events.Events[0]; {
					case e.OnError.Repeat:
						events = state.Events.Events
						log.Debug().Str("cluster", cluster).Msgf("rescheduled for a retry of previously failed task with ID %q.", e.Id)
					case len(e.OnError.Rollback) > 0:
						events = e.OnError.Rollback
						log.Debug().Str("cluster", cluster).Msgf("rescheduled for a rollback with task ID %q of previous failed task with ID %q.", e.OnError.Rollback[0].Id, e.Id)
					default:
						log.Debug().Str("cluster", cluster).Msgf("has not been rescheduled for a retry on failure")
						reschedule = false // no retry strategy on error.
					}

					if reschedule {
						break
					}
				}
			}

			ir, e, err := rollingUpdate(state.Current, state.Desired)
			if err != nil {
				return false, err
			}

			events = append(events, e...)
			if len(events) != 0 {
				log.Debug().Str("cluster", cluster).Msgf("[%d] rolling updates scheduled for k8s cluster, to be performed before building the actual desired state, starting with task with ID %q.", len(events), events[0].Id)
				// First we will let claudie to work on the rolling update
				// to have the latest versions of the terraform manifests.
				// After that the manifest will be rescheduled again
				// to handle the diff between the new current state (with
				// updated terraform files) and the desired state as specified
				// in the Manifest.
				reschedule = true
				// We set the desired state to the intermediate desired state which is the same as the
				// current state but with updated templates for k8s cluster. After this state is build
				// by the builder the config will be rescheduled again to actually reflect the changes
				// made. (if any by the user).
				state.Desired = ir
				break
			}

			ir, e, err = rollingUpdateLBs(state.Current, state.Desired)
			if err != nil {
				return false, err
			}

			events = append(events, e...)
			if len(events) > 0 {
				log.Debug().Str("cluster", cluster).Msgf("[%d] rolling updates scheduled for attached lb clusters, to be performed before building the actual desired state, starting with task with ID %q.", len(events), events[0].Id)
				reschedule = true
				state.Desired = ir
				break
			}

			events = append(events, Diff(
				state.Current.K8S,
				state.Desired.K8S,
				state.Current.GetLoadBalancers().GetClusters(),
				state.Desired.GetLoadBalancers().GetClusters(),
			)...)

			log.Debug().Str("cluster", cluster).Msgf("Scheduled final [%d] tasks to be worked on to build the desired state", len(events))
		}

		state.Events = &spec.Events{Events: events}
		state.State = &spec.Workflow{Stage: spec.Workflow_NONE, Status: spec.Workflow_DONE}
	}

	db, err := store.ConvertFromGRPC(scheduledGRPC)
	if err != nil {
		return false, fmt.Errorf("failed to convert GRPC representation to database for %q: %w", scheduled.Name, err)
	}

	*scheduled = *db
	return reschedule, nil
}

// Diff takes the desired and current state to determine the difference and returns
// a number of tasks to be performed in specific order. It is expected that the current state actually represents
// the actual current state of the cluster and the desired state contains relevant data from the current state with
// the requested changes (i.e. deletion, addition of nodes) from the new config changes, (relevant data was transferred
// to desired state).
func Diff(current, desired *spec.K8Scluster, currentLbs, desiredLbs []*spec.LBcluster) []*spec.TaskEvent {
	k8sDynamic, k8sStatic := NodePoolNodes(current)
	lbsDynamic, lbsStatic := LbsNodePoolNodes(currentLbs)

	k8sDiffResult := k8sNodePoolDiff(k8sDynamic, k8sStatic, desired)
	lbsDiffResult := lbsNodePoolDiff(lbsDynamic, lbsStatic, desiredLbs)
	autoscalerConfigUpdated := k8sAutoscalerDiff(current, desired)
	labelsAnnotationsTaintsUpdated := labelsTaintsAnnotationsDiff(current, desired)

	k8sAllDeletedNodes := make(map[string][]string)
	maps.Insert(k8sAllDeletedNodes, maps.All(k8sDiffResult.deletedDynamic))
	maps.Insert(k8sAllDeletedNodes, maps.All(k8sDiffResult.deletedStatic))
	maps.Insert(k8sAllDeletedNodes, maps.All(k8sDiffResult.partialDeletedDynamic))
	maps.Insert(k8sAllDeletedNodes, maps.All(k8sDiffResult.partialDeletedStatic))

	var deletedLoadbalancers []*spec.LBcluster
	for _, current := range currentLbs {
		found := slices.ContainsFunc(desiredLbs, func(bcluster *spec.LBcluster) bool {
			return current.ClusterInfo.Name == bcluster.ClusterInfo.Name
		})
		if !found {
			deletedLoadbalancers = append(deletedLoadbalancers, current)
		}
	}

	var addedLoadBalancers []*spec.LBcluster
	for _, desired := range desiredLbs {
		found := slices.ContainsFunc(currentLbs, func(bcluster *spec.LBcluster) bool {
			return desired.ClusterInfo.Name == bcluster.ClusterInfo.Name
		})
		if !found {
			addedLoadBalancers = append(addedLoadBalancers, desired)
		}
	}

	var events []*spec.TaskEvent

	currProxySettings := current.InstallationProxy
	desiredProxySettings := desired.InstallationProxy

	if currProxySettings.Mode != desiredProxySettings.Mode || currProxySettings.Endpoint != desiredProxySettings.Endpoint {
		// Proxy settings have been set or changed.
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "changing installation proxy settings",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: &spec.K8Scluster{
						ClusterInfo: current.ClusterInfo,
						Kubernetes:  current.Kubernetes,
						Network:     current.Network,
						InstallationProxy: &spec.InstallationProxy{
							Mode:     desired.InstallationProxy.Mode,
							Endpoint: desired.InstallationProxy.Endpoint,
						},
					},
					Lbs: &spec.LoadBalancers{Clusters: currentLbs},
				},
			},
		})
	}

	// will contain also the deleted nodes / nodepools if any.
	ir := craftK8sIR(k8sDiffResult, current, desired)

	if k8sDiffResult.adding {
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "adding nodes to k8s cluster",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: ir,
					Lbs: &spec.LoadBalancers{Clusters: currentLbs}, // keep current lbs
				},
			},
		})
	}

	if targets, deleted := deletedTargetApiNodePools(k8sDiffResult, current, currentLbs); deleted {
		irLbs := lbClone(currentLbs)
		lb := findLbAPIEndpointCluster(irLbs)

		var nextControlNodePool *spec.NodePool
		for _, np := range utils.FindControlNodepools(desired.ClusterInfo.NodePools) {
			if !slices.ContainsFunc(targets, func(s string) bool { return s == np.Name }) {
				nextControlNodePool = np
				break
			}
		}
		// No need to check if nextControlNodepool is nil. Validation of the inputmanifest
		// does not allow for the user to specify an empty list of control nodes
		nameWithoutHash := nextControlNodePool.Name

		// Each dynamic nodepool after the scheduler stage has a hash appended to it.
		// to get the original nodepool name as defined in the input manifest
		// we need to strip the hash.
		if nextControlNodePool.GetDynamicNodePool() != nil {
			nameWithoutHash = nextControlNodePool.Name[:len(nextControlNodePool.Name)-(utils.HashLength+1)] // +1 for '-'
		}

		for _, role := range lb.GetRoles() {
			if role.RoleType == spec.RoleType_ApiServer {
				role.TargetPools = slices.DeleteFunc(role.TargetPools, func(s string) bool { return slices.Contains(targets, s) })
				role.TargetPools = append(role.TargetPools, nameWithoutHash)
				break
			}
		}

		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "loadbalancer target to new control plane nodepool",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: ir,
					Lbs: &spec.LoadBalancers{Clusters: irLbs},
				},
			},
		})
	}

	if endpointNodeDeleted(k8sDiffResult, current) {
		nodePool, node := newAPIEndpointNodeCandidate(desired.ClusterInfo.NodePools)

		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "moving endpoint from old control plane node to a new control plane node",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{Endpoint: &spec.UpdateState_Endpoint{
					Nodepool: nodePool,
					Node:     node,
				}},
			},
		})
	}

	if k8sDiffResult.deleting {
		dn := make(map[string]*spec.DeletedNodes)
		for k, v := range k8sAllDeletedNodes {
			dn[k] = &spec.DeletedNodes{Nodes: v}
		}
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_DELETE,
			Description: "deleting nodes from k8s cluster",
			Task:        &spec.Task{DeleteState: &spec.DeleteState{Nodepools: dn}},
		})
	}

	// at last infrastructure changes. The loadbalancer changes are not "rolling update"
	// as with the changes to the k8s cluster. If nodepools are added and removed, it
	// will be executed in one go.
	//
	// This will move the current state from an intermediate representation (if any)
	// to the desired as given in the manifest.
	lbsChanges := lbsDiffResult.adding || lbsDiffResult.deleting
	lbsChanges = lbsChanges || !proto.Equal(&spec.LoadBalancers{Clusters: currentLbs}, &spec.LoadBalancers{Clusters: desiredLbs})
	lbsChanges = lbsChanges || len(deletedLoadbalancers) > 0 || len(addedLoadBalancers) > 0
	desc := "reconciling infrastructure changes"
	if k8sDiffResult.deleting {
		desc += ", including deletion of infrastructure for deleted dynamic nodes"
	}
	if lbsChanges {
		desc += ", including changes to the loadbalancer infrastructure"
	}
	if lbsChanges || k8sDiffResult.deleting {
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: desc,
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: desired,
					Lbs: &spec.LoadBalancers{Clusters: desiredLbs},
				},
				DeleteState: func() *spec.DeleteState {
					if len(deletedLoadbalancers) > 0 {
						return &spec.DeleteState{Lbs: &spec.LoadBalancers{Clusters: deletedLoadbalancers}}
					}
					return nil
				}(),
			},
		})
	}

	if len(deletedLoadbalancers) > 0 {
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_DELETE,
			Description: "deleting loadbalancer infrastructure",
			Task: &spec.Task{
				DeleteState: &spec.DeleteState{Lbs: &spec.LoadBalancers{Clusters: deletedLoadbalancers}},
			},
		})
	}

	// No-infrastructure related changes.
	if (autoscalerConfigUpdated || labelsAnnotationsTaintsUpdated) && len(events) == 0 {
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "updating autoscaler config",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: desired,
					Lbs: &spec.LoadBalancers{Clusters: desiredLbs},
				},
			},
		})
	}

	return events
}

type lbsNodePoolDiffResult struct {
	adding   bool
	deleting bool
}

func lbsNodePoolDiff(dynamic, static map[string]map[string][]string, desiredLbs []*spec.LBcluster) lbsNodePoolDiffResult {
	result := lbsNodePoolDiffResult{
		adding:   false,
		deleting: false,
	}

	for _, desired := range desiredLbs {
		for current := range dynamic[desired.GetClusterInfo().GetName()] {
			found := slices.ContainsFunc(desired.GetClusterInfo().GetNodePools(), func(pool *spec.NodePool) bool {
				return (pool.GetDynamicNodePool() != nil) && pool.Name == current
			})
			if !found {
				result.deleting = true
			}
		}

		for current := range static[desired.GetClusterInfo().GetName()] {
			found := slices.ContainsFunc(desired.GetClusterInfo().GetNodePools(), func(pool *spec.NodePool) bool {
				return (pool.GetStaticNodePool() != nil) && pool.Name == current
			})
			if !found {
				result.deleting = true
			}
		}

		for _, desiredNps := range desired.GetClusterInfo().GetNodePools() {
			if desiredNps.GetDynamicNodePool() != nil {
				current, ok := dynamic[desired.GetClusterInfo().GetName()][desiredNps.Name]
				if !ok {
					result.adding = true
					continue
				}

				if desiredNps.GetDynamicNodePool().Count > int32(len(current)) {
					result.adding = true
					continue
				}

				if desiredNps.GetDynamicNodePool().Count < int32(len(current)) {
					result.deleting = true
					// we don't need to keep track of which nodes are being deleted
					// as lbs are not part of k8s cluster.
				}
			} else {
				current, ok := static[desired.GetClusterInfo().GetName()][desiredNps.Name]
				if !ok {
					result.adding = true
					continue
				}

				// Node names are transferred over from current state based on the public IP.
				// Thus, at this point we can figure out based on nodes names which were deleted/added
				// see existing_state.go:transferStaticNodes
				for _, dnode := range desiredNps.Nodes {
					found := slices.ContainsFunc(current, func(s string) bool { return s == dnode.Name })
					if !found {
						result.adding = true
					}
				}

				// Node names are transferred over from current state based on the public IP.
				// Thus, at this point we can figure out based on nodes names which were deleted/added
				// see existing_state.go:transferStaticNodes
				for _, cnode := range current {
					found := slices.ContainsFunc(desiredNps.Nodes, func(dn *spec.Node) bool { return cnode == dn.Name })
					if !found {
						result.deleting = true
						// we don't need to keep track of which nodes are being deleted
						// as lbs are not part of k8s cluster.
					}
				}
			}
		}
	}

	return result
}

type nodePoolDiffResult struct {
	partialDeletedDynamic map[string][]string
	partialDeletedStatic  map[string][]string
	deletedDynamic        map[string][]string
	deletedStatic         map[string][]string
	adding                bool
	deleting              bool
}

// k8sNodePoolDiff calculates difference between desired nodepools and current nodepools in a k8s cluster.
func k8sNodePoolDiff(dynamic, static map[string][]string, desiredCluster *spec.K8Scluster) nodePoolDiffResult {
	result := nodePoolDiffResult{
		partialDeletedDynamic: map[string][]string{},
		partialDeletedStatic:  map[string][]string{},
		deletedStatic:         map[string][]string{},
		deletedDynamic:        map[string][]string{},
		adding:                false,
		deleting:              false,
	}

	// look for whole dynamic nodepools deleted
	for currentNodePool := range dynamic {
		found := slices.ContainsFunc(desiredCluster.GetClusterInfo().GetNodePools(), func(pool *spec.NodePool) bool {
			return (pool.GetDynamicNodePool() != nil) && pool.Name == currentNodePool
		})
		if !found {
			result.deleting = true
			result.deletedDynamic[currentNodePool] = dynamic[currentNodePool]
		}
	}

	// look for whole static nodepools deleted
	for currentNodePool := range static {
		found := slices.ContainsFunc(desiredCluster.GetClusterInfo().GetNodePools(), func(pool *spec.NodePool) bool {
			return (pool.GetStaticNodePool() != nil) && pool.Name == currentNodePool
		})
		if !found {
			result.deleting = true
			result.deletedStatic[currentNodePool] = static[currentNodePool]
		}
	}

	// either both in current/desired but counts may differ or only in desired.
	for _, desired := range desiredCluster.GetClusterInfo().GetNodePools() {
		if desired.GetDynamicNodePool() != nil {
			current, ok := dynamic[desired.Name]
			if !ok {
				// not in current state, adding.
				result.adding = true
				continue
			}

			if desired.GetDynamicNodePool().Count > int32(len(current)) {
				result.adding = true
				continue
			}

			if desired.GetDynamicNodePool().Count < int32(len(current)) {
				result.deleting = true

				// chose nodes to delete.
				toDelete := int(int32(len(current)) - desired.GetDynamicNodePool().Count)
				for i := len(current) - 1; i >= len(current)-toDelete; i-- {
					result.partialDeletedDynamic[desired.Name] = append(result.partialDeletedDynamic[desired.Name], current[i])
				}
			}
		} else {
			current, ok := static[desired.Name]
			if !ok {
				// not in current state, adding.
				result.adding = true
				continue
			}

			// Node names are transferred over from current state based on the public IP.
			// Thus, at this point we can figure out based on nodes names which were deleted/added
			// see existing_state.go:transferStaticNodes
			for _, dnode := range desired.Nodes {
				found := slices.ContainsFunc(current, func(s string) bool { return s == dnode.Name })
				if !found {
					result.adding = true
				}
			}

			// Node names are transferred over from current state based on the public IP.
			// Thus, at this point we can figure out based on nodes names which were deleted/added
			// see existing_state.go:transferStaticNodes
			for _, cnode := range current {
				found := slices.ContainsFunc(desired.Nodes, func(dn *spec.Node) bool { return cnode == dn.Name })
				if !found {
					result.deleting = true
					result.partialDeletedStatic[desired.Name] = append(result.partialDeletedStatic[desired.Name], cnode)
				}
			}
		}
	}
	return result
}

// NodePoolNodes returns the current nodes for the dynamic and static nodepools.
func NodePoolNodes(cluster *spec.K8Scluster) (map[string][]string, map[string][]string) {
	dynamic, static := make(map[string][]string), make(map[string][]string)

	for _, nodePool := range cluster.GetClusterInfo().GetNodePools() {
		if nodePool.GetDynamicNodePool() != nil {
			for _, node := range nodePool.Nodes {
				dynamic[nodePool.Name] = append(dynamic[nodePool.Name], node.Name)
			}
		}
		if nodePool.GetStaticNodePool() != nil {
			for _, node := range nodePool.Nodes {
				static[nodePool.Name] = append(static[nodePool.Name], node.Name)
			}
		}
	}

	return dynamic, static
}

func LbsNodePoolNodes(clusters []*spec.LBcluster) (map[string]map[string][]string, map[string]map[string][]string) {
	dynamic, static := make(map[string]map[string][]string), make(map[string]map[string][]string)

	for _, cluster := range clusters {
		dynamic[cluster.ClusterInfo.Name] = make(map[string][]string)
		static[cluster.ClusterInfo.Name] = make(map[string][]string)

		for _, nodepool := range cluster.GetClusterInfo().GetNodePools() {
			if nodepool.GetDynamicNodePool() != nil {
				for _, node := range nodepool.Nodes {
					dynamic[cluster.ClusterInfo.Name][nodepool.Name] = append(dynamic[cluster.ClusterInfo.Name][nodepool.Name], node.Name)
				}
			}
			if nodepool.GetStaticNodePool() != nil {
				for _, node := range nodepool.Nodes {
					static[cluster.ClusterInfo.Name][nodepool.Name] = append(static[cluster.ClusterInfo.Name][nodepool.Name], node.Name)
				}
			}
		}
	}

	return dynamic, static
}

func lbClone(desiredLbs []*spec.LBcluster) []*spec.LBcluster {
	var result []*spec.LBcluster
	for _, lb := range desiredLbs {
		result = append(result, proto.Clone(lb).(*spec.LBcluster))
	}
	return result
}

func craftK8sIR(k8sDiffResult nodePoolDiffResult, current, desired *spec.K8Scluster) *spec.K8Scluster {
	// Build the Intermediate Representation such that no deletion occurs in desired cluster.
	ir := proto.Clone(desired).(*spec.K8Scluster)

	clusterID := utils.GetClusterID(desired.ClusterInfo)

	k := slices.Collect(maps.Keys(k8sDiffResult.partialDeletedDynamic))
	slices.Sort(k)

	for _, nodepool := range k {
		inp := utils.GetNodePoolByName(nodepool, ir.ClusterInfo.NodePools)
		cnp := utils.GetNodePoolByName(nodepool, current.ClusterInfo.NodePools)

		log.Debug().Str("cluster", clusterID).Msgf("nodes from dynamic nodepool %q were partially deleted, crafting ir to include them", nodepool)
		inp.GetDynamicNodePool().Count = cnp.GetDynamicNodePool().Count
		fillDynamicNodes(clusterID, cnp, inp)
	}

	k = slices.Collect(maps.Keys(k8sDiffResult.partialDeletedStatic))
	slices.Sort(k)

	for _, nodepool := range k {
		log.Debug().Str("cluster", clusterID).Msgf("nodes from static nodepool %q were partially deleted, crafting ir to include them", nodepool)
		inp := utils.GetNodePoolByName(nodepool, ir.ClusterInfo.NodePools)
		cnp := utils.GetNodePoolByName(nodepool, current.ClusterInfo.NodePools)

		is := inp.GetStaticNodePool()
		cs := cnp.GetStaticNodePool()

		maps.Insert(is.NodeKeys, maps.All(cs.NodeKeys))
		transferStaticNodes(clusterID, cnp, inp)

		for _, cn := range cnp.Nodes {
			if slices.Contains(k8sDiffResult.partialDeletedStatic[nodepool], cn.Name) {
				inp.Nodes = append(inp.Nodes, cn)
			}
		}
	}

	deletedNodePools := make(map[string][]string)
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.deletedDynamic))
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.deletedStatic))

	k = slices.Collect(maps.Keys(deletedNodePools))
	slices.Sort(k)

	for _, nodepool := range k {
		log.Debug().Str("cluster", clusterID).Msgf("nodepool %q  deleted, crafting ir to include it", nodepool)
		np := utils.GetNodePoolByName(nodepool, current.ClusterInfo.NodePools)
		ir.ClusterInfo.NodePools = append(ir.ClusterInfo.NodePools, np)
	}

	return ir
}

func endpointNodeDeleted(k8sDiffResult nodePoolDiffResult, current *spec.K8Scluster) bool {
	deletedNodePools := make(map[string][]string)
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.deletedDynamic))
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.deletedStatic))

	for nodepool := range deletedNodePools {
		np := utils.GetNodePoolByName(nodepool, current.ClusterInfo.NodePools)
		if _, err := utils.FindAPIEndpointNode([]*spec.NodePool{np}); err == nil {
			return true
		}
	}

	clear(deletedNodePools)
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.partialDeletedDynamic))
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.partialDeletedStatic))

	for nodepool, nodes := range deletedNodePools {
		np := utils.GetNodePoolByName(nodepool, current.ClusterInfo.NodePools)
		for _, deleted := range nodes {
			i := slices.IndexFunc(np.Nodes, func(node *spec.Node) bool { return node.Name == deleted })
			if i < 0 {
				continue
			}
			if np.Nodes[i].NodeType == spec.NodeType_apiEndpoint {
				return true
			}
		}
	}

	return false
}

func deletedTargetApiNodePools(k8sDiffResult nodePoolDiffResult, current *spec.K8Scluster, currentLbs []*spec.LBcluster) ([]string, bool) {
	deletedNodePools := make(map[string][]string)
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.deletedDynamic))
	maps.Insert(deletedNodePools, maps.All(k8sDiffResult.deletedStatic))

	var deleted []*spec.NodePool
	for np := range deletedNodePools {
		deleted = append(deleted, utils.GetNodePoolByName(np, current.ClusterInfo.NodePools))
	}

	return targetPoolsDeleted(currentLbs, deleted)
}

func newAPIEndpointNodeCandidate(desired []*spec.NodePool) (string, string) {
	for _, np := range desired {
		if np.IsControl {
			return np.Name, np.Nodes[0].Name
		}
	}
	panic("no suitable api endpoint replacement candidate found, malformed state.")
}

// targetPoolsDeleted check whether the LB API cluster target pools are among those that get deleted, if yes returns the names.
func targetPoolsDeleted(current []*spec.LBcluster, nodepools []*spec.NodePool) ([]string, bool) {
	for _, role := range findLbAPIEndpointCluster(current).GetRoles() {
		if role.RoleType != spec.RoleType_ApiServer {
			continue
		}

		var matches []string
		for _, targetPool := range role.TargetPools {
			idx := slices.IndexFunc(nodepools, func(pool *spec.NodePool) bool {
				name, _ := utils.MatchNameAndHashWithTemplate(targetPool, pool.Name)
				return name == targetPool
			})
			if idx >= 0 {
				matches = append(matches, nodepools[idx].Name)
			}
		}

		if len(matches) == len(role.TargetPools) {
			return matches, true
		}
	}
	return nil, false
}

func findLbAPIEndpointCluster(current []*spec.LBcluster) *spec.LBcluster {
	for _, lb := range current {
		if utils.HasAPIServerRole(lb.GetRoles()) {
			return lb
		}
	}
	return nil
}

func k8sAutoscalerDiff(current, desired *spec.K8Scluster) bool {
	cnp := make(map[string]*spec.DynamicNodePool)
	for _, np := range current.GetClusterInfo().GetNodePools() {
		if dyn := np.GetDynamicNodePool(); dyn != nil {
			cnp[np.Name] = dyn
		}
	}

	for _, np := range desired.GetClusterInfo().GetNodePools() {
		if dyn := np.GetDynamicNodePool(); dyn != nil {
			if prev, ok := cnp[np.Name]; ok {
				equal := proto.Equal(prev.AutoscalerConfig, dyn.AutoscalerConfig)
				if !equal {
					return true
				}
			}
		}
	}

	return false
}

func labelsTaintsAnnotationsDiff(current, desired *spec.K8Scluster) bool {
	cnp := make(map[string]*spec.NodePool)
	for _, np := range current.GetClusterInfo().GetNodePools() {
		cnp[np.Name] = np
	}

	for _, np := range desired.GetClusterInfo().GetNodePools() {
		if prev, ok := cnp[np.Name]; ok {
			if !reflect.DeepEqual(prev.Annotations, np.Annotations) {
				return true
			}
			if !reflect.DeepEqual(prev.Labels, np.Labels) {
				return true
			}
			if !reflect.DeepEqual(prev.Taints, np.Taints) {
				return true
			}
		}
	}
	return false
}
