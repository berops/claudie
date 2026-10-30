package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

const (
	createClusterPlaybook = "../../ansible-playbooks/create-cluster.yml"

	// kubeconfigFileName is the name of the file, within the cluster directory,
	// into which the playbook fetches the admin kubeconfig of the created cluster.
	kubeconfigFileName = "kubeconfig"
)

// CreateCluster bootstraps a new Kubernetes cluster with kubeadm on the
// nodes of the kubernetes cluster from the create task.
func CreateCluster(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Creating Kubernetes cluster")

	create, ok := tracker.Task.Do.(*spec.Task_Create)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to create kubernetes cluster, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	k8s := create.Create.K8S
	lbs := create.Create.LoadBalancers

	if err := createCluster(k8s, processLimit); err != nil {
		logger.Err(err).Msg("Failed to create a Kubernetes cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	// Mark all of the nodes with status Joined.
	for _, np := range k8s.ClusterInfo.NodePools {
		for _, n := range np.Nodes {
			n.Status = spec.NodeStatus_Joined
		}
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Loadbalancers(lbs...)
	update.Commit()

	logger.Info().Msg("Finished creating Kubernetes cluster")
}

// createCluster runs the create-cluster playbook against all nodes of the cluster
// and stores the kubeconfig of the created cluster in the passed in cluster.
func createCluster(cluster *spec.K8Scluster, processLimit *semaphore.Weighted) error {
	clusterID := cluster.ClusterInfo.Id()

	// Directory where files (required by Ansible) will be generated.
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)),
	)

	// Ansible resolves relative paths against the playbook directory, not against
	// the directory it is executed from, thus the playbook is given an absolute path
	// to fetch the kubeconfig into.
	kubeconfigPath, err := filepath.Abs(filepath.Join(clusterDirectory, kubeconfigFileName))
	if err != nil {
		return fmt.Errorf("failed to determine the kubeconfig path for %s : %w", clusterDirectory, err)
	}

	flags, err := createClusterFlags(cluster, kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to determine playbook variables for %s : %w", clusterID, err)
	}

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

	// The inventory references the per-nodepool/per-node .pem files,
	// they need to exist in the cluster directory.
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

	if err := utils.GenerateInventoryFile(templates.KubernetesInventoryTemplate, clusterDirectory, idata); err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          createClusterPlaybook,
		Inventory:         utils.InventoryFileName,
		Flags:             flags,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Create cluster - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	// The playbook fetches the admin kubeconfig from the first control node,
	// the later stages (kuber) operate on the cluster through it.
	kubeconfig, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error while reading the kubeconfig fetched by the playbook for %s : %w", clusterID, err)
	}

	if len(kubeconfig) == 0 {
		return fmt.Errorf("the kubeconfig fetched by the playbook for %s is empty", clusterID)
	}

	cluster.Kubeconfig = string(kubeconfig)

	return nil
}

// createClusterFlags builds the --extra-vars passed to the create-cluster playbook.
// Only the kubernetes version and the (controller side) path for the fetched kubeconfig
// are passed in, the remaining variables are initialized within the playbook.
func createClusterFlags(cluster *spec.K8Scluster, kubeconfigPath string) (string, error) {
	if nodepools.FirstControlNode(cluster.ClusterInfo.NodePools) == nil {
		return "", errors.New("cluster has no control plane nodes")
	}

	version := strings.TrimPrefix(strings.TrimSpace(cluster.Kubernetes), "v")
	if version == "" {
		return "", errors.New("cluster has no kubernetes version set")
	}

	return fmt.Sprintf("--extra-vars \"kubernetes_version=%s kubeconfig_dest=%s\"", version, kubeconfigPath), nil
}
