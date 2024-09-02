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
	defaulHttpProxyMode     = "default"
	noProxyDefault          = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12" // 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
	noProxyPlaybookFilePath = "../../ansible-playbooks/update-noproxy-envs.yml"
)

type (
	noProxyInventoryFileParameters struct {
		K8sNodepools NodePools
		ClusterID    string
		NoProxy      string
	}

	NodePools struct {
		Dynamic []*spec.NodePool
		Static  []*spec.NodePool
	}
)

func (u *Usecases) UpdateNoProxyEnvs(request *pb.UpdateNoProxyEnvsRequest) (*pb.UpdateNoProxyEnvsResponse, error) {
	if request.Current == nil {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	hasHetznerNodeFlag := hasHetznerNode(request.Desired.ClusterInfo)
	httpProxyMode := commonUtils.GetEnvDefault("HTTP_PROXY_MODE", defaulHttpProxyMode)
	// Changing NO_PROXY and no_proxy env variables is necessary only when
	// HTTP Proxy mode isn't "off" and cluster has a Hetzner node or cluster is being build using HTTP proxy
	if httpProxyMode == "off" || (httpProxyMode == "default" && !hasHetznerNodeFlag) {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	nodesChangedFlag := nodesChanged(request.Current.ClusterInfo, request.Desired.ClusterInfo)
	// At least one node in the cluster has to be changed to perform the update of NO_PROXY and no_proxy env variables.
	if !nodesChangedFlag {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	log.Info().Msgf("Updating NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateNoProxyEnvs(request.Current.ClusterInfo, request.Desired.ClusterInfo, request.DesiredLbs, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("Failed to update NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
			request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
}

func hasHetznerNode(desiredK8sClusterInfo *spec.ClusterInfo) bool {
	desiredNodePools := desiredK8sClusterInfo.GetNodePools()
	for _, np := range desiredNodePools {
		if np.GetDynamicNodePool() != nil && np.GetDynamicNodePool().Provider.CloudProviderName == "hetzner" {
			return true
		}
	}

	return false
}

func nodesChanged(currentK8sClusterInfo, desiredK8sClusterInfo *spec.ClusterInfo) bool {
	currNodePool := currentK8sClusterInfo.GetNodePools()
	desiredNodePool := desiredK8sClusterInfo.GetNodePools()

	currStateNodesNum := 0
	currStateIpMap := make(map[string]bool)
	for _, np := range currNodePool {
		for _, node := range np.Nodes {
			currStateIpMap[node.Public] = true
			currStateIpMap[node.Private] = true
			currStateNodesNum += 1
		}
	}

	desiredStateNodesNum := 0
	for _, np := range desiredNodePool {
		for _, node := range np.Nodes {
			desiredStateNodesNum += 1
			_, publicIPExists := currStateIpMap[node.Public]
			_, privateIPExists := currStateIpMap[node.Private]

			// Node in at least one nodepool was changed and no proxy env variables have to be changed.
			if !publicIPExists || !privateIPExists {
				return true
			}
		}
	}

	// Returns true if at lease one node was added or removed from the cluster.
	// Otherwise returns false.
	return desiredStateNodesNum != currStateNodesNum
}

// updateNoProxyEnvs handles the case where Claudie adds/removes node to/from the cluster.
// Public and private IPs of this node must be added to the NO_PROXY and no_proxy env variables in
// kube-proxy DaemonSet and static pods.
func updateNoProxyEnvs(currentK8sClusterInfo, desiredK8sClusterInfo *spec.ClusterInfo, desiredLbs []*spec.LBcluster, spawnProcessLimit chan struct{}) error {
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

	// TODO: add LBCluster hostnames and public + private IPs
	noProxyList := createNoProxyList(desiredK8sClusterInfo.GetNodePools(), desiredLbs)
	if err := utils.GenerateInventoryFile(templates.NoProxyEnvsInventoryTemplate, clusterDirectory, noProxyInventoryFileParameters{
		K8sNodepools: NodePools{
			Dynamic: commonUtils.GetCommonDynamicControlPlaneNodes(currentK8sClusterInfo.NodePools, desiredK8sClusterInfo.NodePools),
			Static:  commonUtils.GetCommonStaticControlPlaneNodes(currentK8sClusterInfo.NodePools, desiredK8sClusterInfo.NodePools),
		},
		ClusterID: clusterID,
		NoProxy:   noProxyList,
	}); err != nil {
		return fmt.Errorf("failed to generate inventory file for updating the no proxy envs using playbook in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          noProxyPlaybookFilePath,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: spawnProcessLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Running ansible to update NO_PROXY and no_proxy envs - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update NO_PROXY and no_proxy envs in %s : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}

func createNoProxyList(desiredNodePools []*spec.NodePool, desiredLbs []*spec.LBcluster) string {
	noProxyList := noProxyDefault

	for _, np := range desiredNodePools {
		for _, node := range np.Nodes {
			noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
		}
	}

	for _, lbCluster := range desiredLbs {
		noProxyList = fmt.Sprintf("%s,%s", noProxyList, lbCluster.Dns.Hostname)
		for _, np := range lbCluster.ClusterInfo.NodePools {
			for _, node := range np.Nodes {
				noProxyList = fmt.Sprintf("%s,%s,%s", noProxyList, node.Private, node.Public)
			}
		}
	}

	// if "svc" isn't in noProxyList the admission webhooks will fail, because they will be routed to proxy
	noProxyList = fmt.Sprintf("%s,%s,", noProxyList, "svc")

	return noProxyList
}
