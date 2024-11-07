package utils

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
)

const apiChangePlaybookFilePath = "../../ansible-playbooks/apiEndpointChange.yml"

// ChangeAPIEndpoint will change the kubeadm configuration.
// It will set the Api endpoint of the cluster to the public IP of the
// newly selected ApiEndpoint node.
func ChangeAPIEndpoint(clusterName, oldEndpoint, newEndpoint, directory string, proxyEnvs *spec.ProxyEnvs, spawnProcessLimit chan struct{}) error {
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

// FindCurrentAPIServerTypeLBCluster finds the current API server type LB cluster.
func FindCurrentAPIServerTypeLBCluster(lbClusters []*LBClusterData) *LBClusterData {
	for _, lbClusterData := range lbClusters {
		if lbClusterData.CurrentLbCluster != nil {
			if utils.HasAPIServerRole(lbClusterData.CurrentLbCluster.Roles) && lbClusterData.CurrentLbCluster.Dns != nil {
				return lbClusterData
			}
		}
	}

	return nil
}
