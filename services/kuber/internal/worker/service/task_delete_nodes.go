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

	d, ok := action.Update.Delta.(*spec.Update_KDeleteNodes)
	if !ok {
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not for deleeting k8s nodes, ignoring", action.Update.Delta)
		return
	}

	var master, worker []string

	k8s := action.Update.State.K8S
	np := nodepools.FindByName(d.KDeleteNodes.Nodepool, k8s.ClusterInfo.NodePools)
	if np == nil {
		logger.
			Warn().
			Msgf("Received valid task for deleting nodes, but the nodepool %q from which nodes are "+
				"to be deleted is missing from the provided state, scheduling a deletion of one of the "+
				"specified nodes without propagating the update back", d.KDeleteNodes.Nodepool)

		if len(d.KDeleteNodes.Nodes) < 1 {
			return
		}

		fullname := d.KDeleteNodes.Nodes[0]
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
			master = append(master, fullname)
			logger.Info().Msgf("Deleting %v control nodes", len(master))
		} else {
			worker = append(worker, fullname)
			logger.Info().Msgf("Deleting %v worker nodes", len(worker))
		}

		if err := deleteNodes(logger, master, worker, k8s); err != nil {
			logger.Err(err).Msg("Failed to delete nodes")
			tracker.Diagnostics.Push(err)
			return
		}

		return
	}

	if np.IsControl {
		master = append(master, d.KDeleteNodes.Nodes...)
		logger.Info().Msgf("Deleting %v control nodes", len(master))
	} else {
		worker = append(worker, d.KDeleteNodes.Nodes...)
		logger.Info().Msgf("Deleting %v worker nodes", len(worker))
	}

	if len(master) == 0 && len(worker) == 0 {
		return
	}

	if err := deleteNodes(logger, master, worker, k8s); err != nil {
		logger.Err(err).Msg("Failed delete nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	if d.KDeleteNodes.WithNodePool {
		k8s.ClusterInfo.NodePools = nodepools.DeleteByName(k8s.ClusterInfo.NodePools, d.KDeleteNodes.Nodepool)
	} else {
		nodepools.DeleteNodes(np, d.KDeleteNodes.Nodes)
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.Info().Msg("Nodes successfully deleted")
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

	var description struct {
		Items []nodeOutput `json:"items"`
	}

	out, err := kc.KubectlGet("nodes", "-ojson")
	if err != nil {
		return false, fmt.Errorf("failed to output nodes: %w", err)
	}

	if err := json.Unmarshal(out, &description); err != nil {
		return false, fmt.Errorf("failed to unmarshal node output: %w", err)
	}

	for _, i := range description.Items {
		if i.Metadata.Name == name {
			_, ok := i.Metadata.Labels["node-role.kubernetes.io/control-plane"]
			return ok, nil
		}
	}

	return false, fmt.Errorf("node %q not found in kubectl get nodes output", name)
}

func deleteNodes(logger zerolog.Logger, master, worker []string, k8s *spec.K8Scluster) error {
	deleter, err := nodes.NewDeleter(master, worker, k8s)
	if err != nil {
		return err
	}

	return deleter.DeleteNodes(logger)
}
