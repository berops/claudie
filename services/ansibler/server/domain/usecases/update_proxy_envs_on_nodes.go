package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

const (
	populateProxy = "../../ansible-playbooks/proxy/populate-proxy-envs.yml"
	removeProxy   = "../../ansible-playbooks/proxy/remove-proxy-envs.yml"
)

func (u *Usecases) UpdateProxyEnvsOnNodes(request *pb.UpdateProxyEnvsOnNodesRequest) (*pb.UpdateProxyEnvsOnNodesResponse, error) {
	if request.ProxyEnvs.GetOp() == spec.ProxyOp_NONE {
		return &pb.UpdateProxyEnvsOnNodesResponse{}, nil
	}
	// Update Proxy envs in package tool to install packages using
	// proxy and include setting the proxy to relevant k8s
	// services.
	// NOTE: that the changed proxy settings for the k8s
	// will not take effect until they are properly configured
	// with the update_envs_k8s_services call.
	log.Info().Msgf("Updating proxy envs for nodes in cluster %s project %s", request.Desired.ClusterInfo.Name, request.ProjectName)
	if err := updateProxyEnvsOnNodes(request.Desired.ClusterInfo, request.ProxyEnvs, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("failed to update proxy envs for nodes in cluster %s project %s", request.Desired.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Successfully updated proxy envs for nodes in cluster %s project %s", request.Desired.ClusterInfo.Name, request.ProjectName)
	return &pb.UpdateProxyEnvsOnNodesResponse{}, nil
}

func updateProxyEnvsOnNodes(desiredK8sClusterInfo *spec.ClusterInfo, proxyEnvs *spec.ProxyEnvs, processLimit *semaphore.Weighted) error {
	clusterID := desiredK8sClusterInfo.Id()

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)))
	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(desiredK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(desiredK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	if err := utils.GenerateInventoryFile(templates.ProxyEnvsInventoryTemplate, clusterDirectory, utils.ProxyInventoryFileParameters{
		K8sNodepools: utils.NodePools{
			Dynamic: nodepools.Dynamic(desiredK8sClusterInfo.NodePools),
			Static:  nodepools.Static(desiredK8sClusterInfo.NodePools),
		},
		ClusterID:    clusterID,
		NoProxyList:  proxyEnvs.NoProxyList,
		HttpProxyUrl: proxyEnvs.HttpProxyUrl,
	}); err != nil {
		return fmt.Errorf("failed to generate inventory file for updating proxy envs in /etc/environment using playbook in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	switch proxyEnvs.Op {
	case spec.ProxyOp_MODIFIED:
		ansible.Playbook = populateProxy
	case spec.ProxyOp_OFF:
		ansible.Playbook = removeProxy
	default:
		return fmt.Errorf("unrecognized proxy operation: %v", proxyEnvs.Op.String())
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs in /etc/environment - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs /etc/environment in %s : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}
