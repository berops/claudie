package service

import (
	"fmt"
	"os"
	"path/filepath"

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

// UpdateApiEndpint moves the api endpoint from the current
// node on the kubernetes cluster to the requested node,
func UpdateApiEndpoint(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Updating Api endpoint")

	update, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to update api endpoint, assuming task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	change, ok := update.Update.Delta.(*spec.Update_K8SApiEndpoint)
	if !ok {
		logger.
			Warn().
			Msgf("Received update task with delta %T while wanting to update api endpoint, assuming task was misscheduled, ignoring", update.Update.Delta)
		return
	}

	k8s := update.Update.State.K8S
	if err := updateApiEndpoint(change.K8SApiEndpoint, k8s, processLimit); err != nil {
		logger.Err(err).Msg("Failed to update api endpoint on cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	u := tracker.Result.Update()
	u.Kubernetes(k8s)
	u.Commit()

	logger.Info().Msg("Successfully updated Api endpoint")
}

// updateAPIEndpoint handles the case where the ApiEndpoint node is removed from
// the desired state. Thus, a new control node needs to be selected among the existing
// control nodes. This new control node will then represent the ApiEndpoint of the cluster.
func updateApiEndpoint(
	endpoint *spec.Update_K8SOnlyApiEndpoint,
	cluster *spec.K8Scluster,
	processLimit *semaphore.Weighted,
) error {
	clusterID := cluster.ClusterInfo.Id()
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

	dyn := nodepools.Dynamic(cluster.ClusterInfo.NodePools)
	stc := nodepools.Static(cluster.ClusterInfo.NodePools)

	if err := nodepools.DynamicGenerateKeys(dyn, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(stc, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	idata := KubernetesInventoryParameters{
		K8sNodepools: NodePools{
			Dynamic: dyn,
			Static:  stc,
		},
		ClusterID: clusterID,
	}

	err := utils.GenerateInventoryFile(templates.KubernetesInventoryTemplate, clusterDirectory, idata)
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	_, apiEndpointNode := nodepools.FindApiEndpoint(cluster.ClusterInfo.NodePools)
	if apiEndpointNode == nil {
		return fmt.Errorf("current state cluster doesn't have api endpoint as a control plane node")
	}

	np := nodepools.FindByName(endpoint.Nodepool, cluster.ClusterInfo.NodePools)
	if np == nil {
		return fmt.Errorf("no nodepool %q found within current state", endpoint.Nodepool)
	}

	var newEndpointNode *spec.Node

	for _, node := range np.Nodes {
		if node.Name == endpoint.Node {
			newEndpointNode = node
			break
		}
	}

	if newEndpointNode == nil {
		return fmt.Errorf("no node %q within nodepool %q found in current state", endpoint.Node, endpoint.Nodepool)
	}

	// update the current state
	apiEndpointNode.NodeType = spec.NodeType_master
	newEndpointNode.NodeType = spec.NodeType_apiEndpoint

	newApiEndpoint := newEndpointNode.GetPublic()

	return utils.ChangeAPIEndpoint(
		clusterID,
		apiEndpointNode.GetPublic(),
		newApiEndpoint,
		clusterDirectory,
		processLimit,
	)
}
