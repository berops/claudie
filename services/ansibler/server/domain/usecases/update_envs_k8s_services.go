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
	commitProxy = "../../ansible-playbooks/proxy/commit-proxy-envs-changes.yml"
)

func (u *Usecases) UpdateProxyEnvsK8sServices(request *pb.UpdateProxyEnvsK8SServicesRequest) (*pb.UpdateProxyEnvsK8SServicesResponse, error) {
	skip := request.Current == nil || request.Current.Kubeconfig == ""
	skip = skip || request.ProxyEnvs == nil
	skip = skip || request.ProxyEnvs.GetOp() == spec.ProxyOp_NONE
	if skip {
		// Don't update no proxy envs, when the k8s cluster wasn't build yet or the proxy envs are not supposed to be updated.
		return &pb.UpdateProxyEnvsK8SServicesResponse{}, nil
	}

	log.Info().
		Msgf("Updating kube-proxy DaemonSet and static pods for cluster %s project %s with new Proxy envs",
			request.Current.ClusterInfo.Name,
			request.ProjectName)

	if err := updateNoProxyEnvsInKubernetes(request.Current.ClusterInfo, request.Desired.ClusterInfo, request.ProxyEnvs, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("failed to update proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
			request.Current.ClusterInfo.Name,
			request.ProjectName,
		)
	}

	log.Info().
		Msgf("Updated proxy envs for kube-proxy DaemonSet and static pods for cluster %s project %s successfully",
			request.Current.ClusterInfo.Name,
			request.ProjectName)

	return &pb.UpdateProxyEnvsK8SServicesResponse{}, nil
}

// updateNoProxyEnvsInKubernetes updates NO_PROXY and no_proxy envs across k8s services on nodes.
func updateNoProxyEnvsInKubernetes(currentK8sClusterInfo, desiredK8sClusterInfo *spec.ClusterInfo, proxyEnvs *spec.ProxyEnvs, processLimit *semaphore.Weighted) error {
	clusterID := currentK8sClusterInfo.Id()

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)))
	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(currentK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(currentK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	if err := utils.GenerateInventoryFile(templates.ProxyEnvsInventoryTemplate, clusterDirectory, utils.ProxyInventoryFileParameters{
		K8sNodepools: utils.NodePools{
			Dynamic: nodepools.CommonDynamicNodes(currentK8sClusterInfo.NodePools, desiredK8sClusterInfo.NodePools),
			Static:  nodepools.CommonStaticNodes(currentK8sClusterInfo.NodePools, desiredK8sClusterInfo.NodePools),
		},
		ClusterID:    clusterID,
		NoProxyList:  proxyEnvs.NoProxyList,
		HttpProxyUrl: proxyEnvs.HttpProxyUrl,
	}); err != nil {
		return fmt.Errorf("failed to generate inventory file for updating proxy envs using playbook in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          commitProxy,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs in %s : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}
