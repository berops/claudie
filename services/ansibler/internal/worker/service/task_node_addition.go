package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	utils "github.com/berops/claudie/services/ansibler/internal/worker/service/internal"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

const addNodesPlaybook = "../../ansible-playbooks/add-nodes.yml"

// AddNodes joins the nodes added by the update task into the existing Kubernetes cluster
// with kubeadm. The kubeconfig of the cluster does not change.
func AddNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Adding nodes into the Kubernetes cluster")

	task, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to add nodes into the kubernetes cluster, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	// The manager converts the terraformer delta of dynamic nodes into this one once
	// their infrastructure is built, static nodes are scheduled with it directly.
	added := task.Update.GetAddedK8SNodes()
	if len(added.GetNodes()) == 0 {
		logger.
			Warn().
			Msgf("Received update task with delta %T without nodes to add into the kubernetes cluster, assuming the task was misscheduled, ignoring", task.Update.GetDelta())
		return
	}

	k8s := task.Update.State.K8S
	if err := addNodes(k8s, task.Update.State.LoadBalancers, added, processLimit); err != nil {
		logger.Err(err).Msg("Failed to add nodes into the Kubernetes cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.Info().Msgf("Finished adding nodes into nodepool %q of the Kubernetes cluster", added.Nodepool)
}

// addNodes joins the added nodes with the add-nodes playbook and marks them as joined. The
// playbook creates the join credentials on a control node that is already part of the cluster.
func addNodes(
	cluster *spec.K8Scluster,
	lbs []*spec.LBcluster,
	added *spec.Update_AddedK8SNodes,
	processLimit *semaphore.Weighted,
) error {
	clusterID := cluster.ClusterInfo.Id()

	np := nodepools.FindByName(added.Nodepool, cluster.ClusterInfo.NodePools)
	if np == nil {
		return fmt.Errorf("nodepool %q with the nodes to add is missing from %s", added.Nodepool, clusterID)
	}

	// Shares the nodes with the cluster, so marking them as joined below updates the cluster.
	newNodes := nodepools.PartialCopyWithNodeFilter(np, added.Nodes)

	existing := existingControlNode(cluster, added.Nodes)
	if existing == nil {
		return fmt.Errorf("%s has no joined control node to create the join credentials on", clusterID)
	}

	// The same endpoint is used by kuber for Cilium, see [clusters.ControlPlaneEndpointHost].
	endpoint, err := clusters.ControlPlaneEndpointHost(cluster, lbs)
	if err != nil {
		return fmt.Errorf("failed to determine the control plane endpoint of %s : %w", clusterID, err)
	}

	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			log.Err(err).Msgf("error while deleting files in %s", clusterDirectory)
		}
	}()

	// The inventory references the per-nodepool/per-node .pem files,
	// they need to exist in the cluster directory.
	nps := []*spec.NodePool{newNodes, existing}
	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(nps), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(nps), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	idata := KubernetesInventoryParameters{
		K8sNodepools: NodePools{
			Dynamic: nodepools.Dynamic([]*spec.NodePool{newNodes}),
			Static:  nodepools.Static([]*spec.NodePool{newNodes}),
		},
		ClusterID:           clusterID,
		ExistingControlNode: existing,
	}

	if err := utils.GenerateInventoryFile(templates.KubernetesInventoryTemplate, clusterDirectory, idata); err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          addNodesPlaybook,
		Inventory:         utils.InventoryFileName,
		Flags:             fmt.Sprintf("--extra-vars \"control_plane_endpoint_host=%s\"", endpoint),
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Add nodes - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	for _, n := range newNodes.Nodes {
		n.Status = spec.NodeStatus_Joined
	}

	return nil
}

// existingControlNode returns a control node that already joined the cluster, in a copy of its
// nodepool holding only that node. Any such node can create the join credentials. The added nodes
// are skipped, they are already marked as joined when the task is retried after this stage succeeded.
func existingControlNode(cluster *spec.K8Scluster, added []string) *spec.NodePool {
	for np := range nodepools.Control(cluster.ClusterInfo.NodePools) {
		for _, n := range np.Nodes {
			if n.Status == spec.NodeStatus_Joined && !slices.Contains(added, n.Name) {
				return nodepools.PartialCopyWithNodeFilter(np, []string{n.Name})
			}
		}
	}
	return nil
}
