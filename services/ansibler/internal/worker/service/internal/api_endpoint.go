package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sync/semaphore"
)

const (
	apiChangePlaybookFilePath         = "../../ansible-playbooks/api-server-endpoint-change.yml"
	apiChangeKubeconfigFileNameFormat = "%s-kubeconfig"
)

// ChangeAPIEndpoint moves the control plane endpoint of the cluster from oldEndpoint to newEndpoint,
// each the DNS hostname of a loadbalancer or the public IP of a control node, without the port.
// The playbook stores newEndpoint in the kubeadm-config ConfigMap, issues the api server
// certificates for it and points the kubeconfigs on the nodes at it. Returns the admin kubeconfig
// of the cluster, which points at newEndpoint.
func ChangeAPIEndpoint(clusterName, oldEndpoint, newEndpoint, directory string, spawnProcessLimit *semaphore.Weighted) (string, error) {
	// Ansible resolves relative paths against the playbook directory, not against
	// the directory it is executed from, thus the playbook is given an absolute path
	// to fetch the kubeconfig into.
	kubeconfigPath, err := filepath.Abs(filepath.Join(directory, fmt.Sprintf(apiChangeKubeconfigFileNameFormat, clusterName)))
	if err != nil {
		return "", fmt.Errorf("failed to determine the kubeconfig path for %s : %w", directory, err)
	}

	ansible := Ansible{
		Playbook:  apiChangePlaybookFilePath,
		Inventory: InventoryFileName,
		Flags: fmt.Sprintf(
			"--extra-vars \"old_control_plane_endpoint_host=%s new_control_plane_endpoint_host=%s kubeconfig_dest=%s\"",
			oldEndpoint, newEndpoint, kubeconfigPath,
		),
		Directory:         directory,
		SpawnProcessLimit: spawnProcessLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", clusterName)); err != nil {
		return "", fmt.Errorf("error while running ansible: %w ", err)
	}

	kubeconfig, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("error while reading the kubeconfig fetched by the playbook for %s : %w", clusterName, err)
	}

	if len(kubeconfig) == 0 {
		return "", fmt.Errorf("the kubeconfig fetched by the playbook for %s is empty", clusterName)
	}

	return string(kubeconfig), nil
}
