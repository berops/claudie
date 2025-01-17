package utils

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/proto/pb/spec"

	"golang.org/x/sync/semaphore"
)

const apiChangePlaybookFilePath = "../../ansible-playbooks/apiEndpointChange.yml"

// ChangeAPIEndpoint will change the kubeadm configuration.
// It will set the Api endpoint of the cluster to the public IP of the
// newly selected ApiEndpoint node.
func ChangeAPIEndpoint(clusterName, oldEndpoint, newEndpoint, directory string, proxyEnvs *spec.ProxyEnvs, spawnProcessLimit *semaphore.Weighted) error {
	proxyEnvs.NoProxyList = strings.Replace(proxyEnvs.NoProxyList, oldEndpoint, newEndpoint, 1)

	ansible := Ansible{
		Playbook:  apiChangePlaybookFilePath,
		Inventory: InventoryFileName,
		Flags: fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s HttpProxyUrl=%s NoProxyList=%s\"",
			newEndpoint, oldEndpoint, proxyEnvs.HttpProxyUrl, proxyEnvs.NoProxyList),
		Directory:         directory,
		SpawnProcessLimit: spawnProcessLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", clusterName)); err != nil {
		return fmt.Errorf("error while running ansible: %w ", err)
	}

	return nil
}
