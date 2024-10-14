package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
)

const (
	proxyPlaybookFilePath = "../../ansible-playbooks/update-proxy-envs.yml"
)

type (
	proxyInventoryFileParameters struct {
		K8sControlPlaneNodepools NodePools
		K8sNodepools             NodePools
		ClusterID                string
		NoProxyList              string
		HttpProxyUrl             string
	}

	NodePools struct {
		Dynamic []*spec.NodePool
		Static  []*spec.NodePool
	}
)

func (u *Usecases) UpdateProxyEnvs(request *pb.UpdateProxyEnvsRequest) (*pb.UpdateProxyEnvsResponse, error) {
	if request.Current == nil {
		return &pb.UpdateProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	log.Info().Msgf("Updating proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateProxyEnvs(request.Current.ClusterInfo, request.Desired.ClusterInfo, request.DesiredLbs, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("Failed to update proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
			request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
}

// updateProxyEnvs updates proxy env variables accordingly
func updateProxyEnvs(currentK8sClusterInfo, desiredK8sClusterInfo *spec.ClusterInfo, desiredLbs []*spec.LBcluster, spawnProcessLimit chan struct{}) error {
	clusterID := commonUtils.GetClusterID(currentK8sClusterInfo)

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, commonUtils.CreateHash(commonUtils.HashLength)))
	if err := commonUtils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := commonUtils.CreateKeysForDynamicNodePools(commonUtils.GetCommonDynamicNodePools(currentK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := commonUtils.CreateKeysForStaticNodepools(commonUtils.GetCommonStaticNodePools(currentK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	httpProxyUrl, noProxyList := utils.GetHttpProxyUrlAndNoProxyList(desiredK8sClusterInfo, desiredLbs)

	if err := utils.GenerateInventoryFile(templates.ProxyEnvsInventoryTemplate, clusterDirectory, proxyInventoryFileParameters{
		K8sControlPlaneNodepools: NodePools{
			Dynamic: commonUtils.GetCommonDynamicControlPlaneNodes(currentK8sClusterInfo.NodePools, desiredK8sClusterInfo.NodePools),
			Static:  commonUtils.GetCommonStaticControlPlaneNodes(currentK8sClusterInfo.NodePools, desiredK8sClusterInfo.NodePools),
		},
		K8sNodepools: NodePools{
			Dynamic: commonUtils.GetCommonDynamicNodePools(currentK8sClusterInfo.NodePools),
			Static:  commonUtils.GetCommonStaticNodePools(currentK8sClusterInfo.NodePools),
		},
		ClusterID:    clusterID,
		NoProxyList:  noProxyList,
		HttpProxyUrl: httpProxyUrl,
	}); err != nil {
		return fmt.Errorf("failed to generate inventory file for updating proxy envs using playbook in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          proxyPlaybookFilePath,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: spawnProcessLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs in %s : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}
