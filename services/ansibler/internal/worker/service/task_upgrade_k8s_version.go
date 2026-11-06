package service

import (
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

const upgradeK8sVersionPlaybook = "../../ansible-playbooks/upgrade-k8s-version.yml"

// UpgradeK8sVersion upgrades the Kubernetes cluster from the update task to the version
// of its delta with kubeadm, one node at a time.
func UpgradeK8sVersion(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Upgrading Kubernetes version")

	task, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to upgrade the kubernetes version, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	upgrade := task.Update.GetUpgradeVersion()
	if upgrade == nil {
		logger.
			Warn().
			Msgf("Received update task with delta %T while wanting to upgrade the kubernetes version, assuming the task was misscheduled, ignoring", task.Update.GetDelta())
		return
	}

	k8s := task.Update.State.K8S
	if err := upgradeK8sVersion(k8s, upgrade.Version, processLimit); err != nil {
		logger.Err(err).Msg("Failed to upgrade the Kubernetes version")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.Info().Msgf("Finished upgrading the Kubernetes version to %s", k8s.Kubernetes)
}

// upgradeK8sVersion runs the upgrade-k8s-version playbook against all nodes of the cluster and
// stores the new version and the kubeconfig fetched by the playbook in the passed in cluster.
// The kubeconfig is fetched again because kubeadm renews its client certificate during the upgrade.
func upgradeK8sVersion(cluster *spec.K8Scluster, version string, processLimit *semaphore.Weighted) error {
	clusterID := cluster.ClusterInfo.Id()

	playbookVersion := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if playbookVersion == "" {
		return fmt.Errorf("no kubernetes version to upgrade %s to", clusterID)
	}

	// Directory where files (required by Ansible) will be generated.
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)),
	)

	// Ansible resolves relative paths against the playbook directory, not against
	// the directory it is executed from, thus the playbook is given an absolute path
	// to fetch the kubeconfig into.
	kubeconfigPath, err := filepath.Abs(filepath.Join(clusterDirectory, fmt.Sprintf(kubeconfigFileNameFormat, cluster.ClusterInfo.Name)))
	if err != nil {
		return fmt.Errorf("failed to determine the kubeconfig path for %s : %w", clusterDirectory, err)
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
		Playbook:  upgradeK8sVersionPlaybook,
		Inventory: utils.InventoryFileName,
		Flags: fmt.Sprintf(
			"--extra-vars \"kubernetes_version=%s kubeconfig_dest=%s\"",
			playbookVersion, kubeconfigPath,
		),
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Upgrade cluster - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	kubeconfig, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error while reading the kubeconfig fetched by the playbook for %s : %w", clusterID, err)
	}

	if len(kubeconfig) == 0 {
		return fmt.Errorf("the kubeconfig fetched by the playbook for %s is empty", clusterID)
	}

	cluster.Kubernetes = version
	cluster.Kubeconfig = string(kubeconfig)

	return nil
}
