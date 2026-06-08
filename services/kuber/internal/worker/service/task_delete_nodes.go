package service

import (
	"encoding/json"
	"fmt"
	"strings"

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

func deleteNodes(logger zerolog.Logger, master, worker []*spec.Node, k8s *spec.K8Scluster) error {
	deleter, err := nodes.NewDeleter(master, worker, k8s)
	if err != nil {
		return err
	}

	return deleter.DeleteNodes(logger)
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

			var master, worker []*spec.Node

			if isControl {
				logger.Info().Msgf("Deleting control node %q", fullname)
				master = append(master, &spec.Node{Name: fullname})
			} else {
				logger.Info().Msgf("Deleting worker node %q", fullname)
				worker = append(worker, &spec.Node{Name: fullname})
			}

			if err := deleteNodes(logger, master, worker, k8s); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}
			// Do not propagate an update in this case as the nodepool is not tracked
			// and the manager service wil refuse the update.
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
	var master, worker []*spec.Node

	switch typ := a.DeletedK8SNodes.Kind.(type) {
	case *spec.Update_DeletedK8SNodes_Partial_:
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

			if isControl {
				logger.Info().Msgf("Deleting control node %q", fullname)
				master = append(master, node)
			} else {
				logger.Info().Msgf("Deleting worker node %q", fullname)
				worker = append(worker, node)
			}

			if err := deleteNodes(logger, master, worker, k8s); err != nil {
				logger.Err(err).Msg("Failed to delete nodes")
				tracker.Diagnostics.Push(err)
				return
			}

			// Do not propagate an update in this case as the nodepool is not tracked
			// and the manager service wil refuse the update
			return
		}

		if np.IsControl {
			master = append(master, typ.Partial.Nodes...)
		} else {
			worker = append(worker, typ.Partial.Nodes...)
		}
	case *spec.Update_DeletedK8SNodes_Whole:
		if typ.Whole.Nodepool.IsControl {
			master = append(master, typ.Whole.Nodepool.Nodes...)
		} else {
			worker = append(worker, typ.Whole.Nodepool.Nodes...)
		}
	}

	if len(master) == 0 && len(worker) == 0 {
		logger.Info().Msg("Received task with no nodes to remove")
		return
	}

	logger.
		Info().
		Msgf("Deleting %v control nodes %v worker nodes", len(master), len(worker))

	if err := deleteNodes(logger, master, worker, k8s); err != nil {
		logger.Err(err).Msg("Failed to delete nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Nodes successfully deleted")
}
