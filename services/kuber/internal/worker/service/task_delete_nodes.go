package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/internal/worker/service/internal/nodes"
	"github.com/rs/zerolog"
)

func DeleteNodes(logger zerolog.Logger, tracker Tracker) {
	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not an update, ignoring", tracker.Task.Do)
		return
	}

	switch typ := action.Update.Delta.(type) {
	case *spec.Update_DeletedK8SNodes_:
		deleteNodesFromCluster(logger, typ, action.Update.State.K8S, tracker)
		return
	case *spec.Update_KDeleteNodes:
		deleteFromState(logger, typ, action.Update.State.K8S, tracker)
		return
	default:
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not supported, ignoring", action.Update.Delta)
		return
	}
}

func deleteFromState(
	logger zerolog.Logger,
	a *spec.Update_KDeleteNodes,
	k8s *spec.K8Scluster,
	tracker Tracker,
) {
	if a.KDeleteNodes.WithNodePool {
		k8s.ClusterInfo.NodePools = nodepools.DeleteByName(k8s.ClusterInfo.NodePools, a.KDeleteNodes.Nodepool)
	} else {
		np := nodepools.FindByName(a.KDeleteNodes.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			// The scheduled deletion of the node from the nodepool is not present
			// in the clusters state. As to why this code is place in this function
			// and also in the [deleteNodesFromCluster] function, the reason is that
			// since these nodes are not in the tracked state when the message is submited
			// back into the manager service it will refuse the update.
			//
			// However since this function is part of a node deletion process we need
			// to handle drifts in the state at this point. For example it may happen
			// that a VM was shutdown and removed from the cluster, but the VM itself
			// may not have been destroyed and came back online at a future point in time
			//
			// which causes a drift in the state and the VM is no longer in the tracked
			// state but claudie schedules a deletion to fix the drift in which case
			// the code in this function handles it.
			logger.
				Warn().
				Msgf("Received valid task for deleting nodes, but the nodepool %q from which nodes are "+
					"to be deleted is missing from the provided state, interpreting this as a drift and "+
					"scheduling a deletion of one of the nodes", a.KDeleteNodes.Nodepool)

			if len(a.KDeleteNodes.Nodes) < 1 {
				return
			}

			fullname := a.KDeleteNodes.Nodes[0]
			strippedName := strings.TrimPrefix(fullname, fmt.Sprintf("%s-", k8s.ClusterInfo.Id()))

			isControl, err := isControlNode(strippedName, k8s.Kubeconfig)
			if err != nil {
				logger.
					Warn().
					Msgf("Failed to determine role for node %q within kubernetes cluster, "+
						"assuming node is no longer part of the cluster", fullname)
				return
			}

			if err := deleteUntrackedNodes(logger, k8s, isControl, []*spec.Node{{Name: fullname}}); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}

			// Do not propagate an update in this case as the nodepool is not tracked
			// and the manager service wil refuse the update. This will also cause
			// for the manager to not consume the update and therefore continue with
			// other stages, if any.
			return
		}
		nodepools.DeleteNodes(np, a.KDeleteNodes.Nodes)
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.
		Info().
		Msgf("Nodes %v Removed from tracked state", a.KDeleteNodes.Nodes)
}

func deleteNodesFromCluster(
	logger zerolog.Logger,
	a *spec.Update_DeletedK8SNodes_,
	k8s *spec.K8Scluster,
	tracker Tracker,
) {
	switch typ := a.DeletedK8SNodes.Kind.(type) {
	case *spec.Update_DeletedK8SNodes_Partial_:
		if len(typ.Partial.Nodes) < 1 {
			logger.Info().Msg("Received task with no nodes to remove")
			return
		}

		np := nodepools.FindByName(typ.Partial.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf("Received valid task for deleting nodes, but the nodepool %q from which nodes are "+
					"to be deleted is missing from the provided state, interpreting this as a drift and "+
					"scheduling a deletion of one of the nodes", typ.Partial.Nodepool)

			if len(typ.Partial.Nodes) < 1 {
				return
			}

			node := typ.Partial.Nodes[0]
			fullname := node.Name
			strippedName := strings.TrimPrefix(fullname, fmt.Sprintf("%s-", k8s.ClusterInfo.Id()))

			isControl, err := isControlNode(strippedName, k8s.Kubeconfig)
			if err != nil {
				logger.
					Warn().
					Msgf("Failed to determine role for node %q within kubernetes cluster, "+
						"assuming node is no longer part of the cluster", fullname)
				return
			}

			if err := deleteUntrackedNodes(logger, k8s, isControl, []*spec.Node{node}); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}

			// Do not propagate an update in this case as the nodepool is not tracked
			// and the manager service wil refuse the update
			return
		}

		switch t := np.Type.(type) {
		case *spec.NodePool_DynamicNodePool:
			logger.
				Info().
				Msgf("Deleting %v dynamic node/s from nodepool %q", len(typ.Partial.Nodes), np.Name)

			if err := deleteDynamicNodes(logger, k8s, np, typ.Partial.Nodes); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}
		case *spec.NodePool_StaticNodePool:
			logger.
				Info().
				Msgf("Deleting %v static node/s from nodepool %q", len(typ.Partial.Nodes), np.Name)

			if err := deleteStaticNodes(logger, k8s, np, typ.Partial.Nodes, typ.Partial.StaticNodeKeys); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}
		default:
			logger.Info().Msgf("Unknown nodepool type %T", t)
			return
		}

		logger.Info().Msg("Nodes successfully deleted")
		return
	case *spec.Update_DeletedK8SNodes_Whole:
		if len(typ.Whole.Nodepool.Nodes) < 1 {
			logger.Info().Msg("Received task with no nodes to remove")
			return
		}

		np := typ.Whole.Nodepool
		switch t := np.Type.(type) {
		case *spec.NodePool_DynamicNodePool:
			logger.
				Info().
				Msgf("Deleting %v dynamic node/s from nodepool %q", len(np.Nodes), np.Name)

			if err := deleteDynamicNodes(logger, k8s, np, np.Nodes); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}
		case *spec.NodePool_StaticNodePool:
			logger.
				Info().
				Msgf("Deleting %v static node/s from nodepool %q", len(np.Nodes), np.Name)

			if err := deleteStaticNodes(logger, k8s, np, np.Nodes, np.GetStaticNodePool().NodeKeys); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}
		default:
			logger.Info().Msgf("Unknown nodepool type %T", t)
			return
		}

		logger.Info().Msg("Nodes successfully deleted")
		return
	default:
		logger.
			Info().
			Msgf("Received unknown task %T for node deletion, skipping", typ)
		return
	}
}

func isControlNode(name string, kubeconfig string) (bool, error) {
	kc := kubectl.Kubectl{
		Kubeconfig:        kubeconfig,
		MaxKubectlRetries: kubectl.NoRetries,
	}

	type nodeOutput struct {
		Metadata struct {
			Name   string `json:"name"`
			Labels map[string]any
		} `json:"metadata"`
	}

	out, err := kc.KubectlGet("nodes", name, "-ojson")
	if err != nil {
		return false, fmt.Errorf("failed to output nodes: %w", err)
	}

	var description nodeOutput
	if err := json.Unmarshal(out, &description); err != nil {
		return false, fmt.Errorf("failed to unmarshal node output: %w", err)
	}

	_, ok := description.Metadata.Labels["node-role.kubernetes.io/control-plane"]
	return ok, nil
}

func deleteUntrackedNodes(logger zerolog.Logger, k8s *spec.K8Scluster, isControl bool, n []*spec.Node) error {
	var master, worker []nodes.NodeInfo
	var ni []nodes.NodeInfo

	for i := range n {
		ni = append(ni, nodes.NodeInfo{
			Fullname: n[i].Name,
			K8sName:  strings.TrimPrefix(n[i].Name, fmt.Sprintf("%s-", k8s.ClusterInfo.Id())),
			// Zero value, treat as unreachable by default.
			Ep: clusters.NodeEndpoint{},
		})
	}

	if isControl {
		master = append(master, ni...)
	} else {
		worker = append(worker, ni...)
	}

	deleter, err := nodes.NewDeleter(master, worker, k8s)
	if err != nil {
		return err
	}
	return deleter.DeleteNodes(logger)
}

func deleteStaticNodes(
	logger zerolog.Logger,
	k8s *spec.K8Scluster,
	np *spec.NodePool,
	n []*spec.Node,
	nodeKeys map[string]string,
) error {
	var master, worker []nodes.NodeInfo
	var ni []nodes.NodeInfo

	for i := range n {
		ni = append(ni, nodes.NodeInfo{
			Fullname: n[i].Name,
			K8sName:  strings.TrimPrefix(n[i].Name, fmt.Sprintf("%s-", k8s.ClusterInfo.Id())),
			Ep: clusters.NodeEndpoint{
				Ip:       n[i].Public,
				Port:     fmt.Sprint(nodepools.NodeSSHPort(np, n[i])),
				Username: nodepools.NodeSSHUsername(n[i]),
				SSHKey:   nodeKeys[n[i].Public],
			},
		})
	}

	if np.IsControl {
		master = append(master, ni...)
	} else {
		worker = append(worker, ni...)
	}

	deleter, err := nodes.NewDeleter(master, worker, k8s)
	if err != nil {
		return err
	}
	return deleter.DeleteNodes(logger)
}

func deleteDynamicNodes(
	logger zerolog.Logger,
	k8s *spec.K8Scluster,
	np *spec.NodePool,
	n []*spec.Node,
) error {
	var master, worker []nodes.NodeInfo
	var ni []nodes.NodeInfo

	for i := range n {
		ni = append(ni, nodes.NodeInfo{
			Fullname: n[i].Name,
			K8sName:  strings.TrimPrefix(n[i].Name, fmt.Sprintf("%s-", k8s.ClusterInfo.Id())),
			Ep: clusters.NodeEndpoint{
				Ip:       n[i].Public,
				Port:     fmt.Sprint(nodepools.NodeSSHPort(np, n[i])),
				Username: nodepools.NodeSSHUsername(n[i]),
				SSHKey:   np.GetDynamicNodePool().PrivateKey,
			},
		})
	}

	if np.IsControl {
		master = append(master, ni...)
	} else {
		worker = append(worker, ni...)
	}

	deleter, err := nodes.NewDeleter(master, worker, k8s)
	if err != nil {
		return err
	}
	return deleter.DeleteNodes(logger)
}
