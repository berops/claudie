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
		MaxKubectlRetries: -1,
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
				Msgf("Nodepool %q not present in current state, assuming it was deleted", a.KDeleteNodes.Nodepool)

			return
		}
		nodepools.DeleteNodes(np, a.KDeleteNodes.Nodes)
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.Info().Msg("Nodes Removed from tracked state")
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
					Err(err).
					Msgf("Failed to determine role for node %q within kubernetes cluster", fullname)
				tracker.Diagnostics.Push(err)
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

			return
		}

		if np.IsControl {
			for _, n := range typ.Partial.Nodes {
				master = append(master, n)
			}
		} else {
			for _, n := range typ.Partial.Nodes {
				worker = append(worker, n)
			}
		}
	case *spec.Update_DeletedK8SNodes_Whole:
		if typ.Whole.Nodepool.IsControl {
			for _, n := range typ.Whole.Nodepool.Nodes {
				master = append(master, n)
			}
		} else {
			for _, n := range typ.Whole.Nodepool.Nodes {
				worker = append(worker, n)
			}
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

	logger.Info().Msg("Nodes successfuly deleted")
}
