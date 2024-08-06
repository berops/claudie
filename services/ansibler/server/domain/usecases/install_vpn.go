package usecases

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog/log"
)

const (
	wireguardPlaybookFilePath = "../../ansible-playbooks/wireguard.yml"
	wireguardUninstall        = "../../ansible-playbooks/wireguard-uninstall.yml"
)

type VPNInfo struct {
	ClusterNetwork string
	// NodepoolsInfos is a slice with each element of type *DesiredClusterNodepoolsInfo.
	// Each element corresponds to a cluster (either a Kubernetes cluster or attached LB clusters).
	NodepoolsInfos []*NodepoolsInfo
}

// InstallVPN installs VPN between nodes in the k8s cluster and lb clusters
func (u *Usecases) InstallVPN(request *pb.InstallRequest) (*pb.InstallResponse, error) {
	logger := log.With().Str("project", request.ProjectName).Str("cluster", request.Desired.ClusterInfo.Name).Logger()
	logger.Info().Msgf("Installing VPN")

	vpnInfo := &VPNInfo{
		ClusterNetwork: request.Desired.Network,
		NodepoolsInfos: []*NodepoolsInfo{
			// Construct and add NodepoolsInfo for the Kubernetes cluster
			{
				Nodepools: utils.NodePools{
					Dynamic: commonUtils.GetCommonDynamicNodePools(request.Desired.ClusterInfo.NodePools),
					Static:  commonUtils.GetCommonStaticNodePools(request.Desired.ClusterInfo.NodePools),
				},
				ClusterID:      commonUtils.GetClusterID(request.Desired.ClusterInfo),
				ClusterNetwork: request.Desired.Network,
			},
		},
	}
	// Construct and add NodepoolsInfo for each of the attached LB clusters
	for _, lbCluster := range request.DesiredLbs {
		vpnInfo.NodepoolsInfos = append(vpnInfo.NodepoolsInfos,
			&NodepoolsInfo{
				Nodepools: utils.NodePools{
					Dynamic: commonUtils.GetCommonDynamicNodePools(lbCluster.ClusterInfo.NodePools),
					Static:  commonUtils.GetCommonStaticNodePools(lbCluster.ClusterInfo.NodePools),
				},
				ClusterID:      commonUtils.GetClusterID(lbCluster.ClusterInfo),
				ClusterNetwork: request.Desired.Network,
			},
		)
	}

	if err := installWireguardVPN(commonUtils.GetClusterID(request.Desired.ClusterInfo), vpnInfo, u.SpawnProcessLimit); err != nil {
		logger.Err(err).Msgf("Error encountered while installing VPN")
		return nil, fmt.Errorf("error encountered while installing VPN for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("VPN was successfully installed")
	return &pb.InstallResponse{Desired: request.Desired, DesiredLbs: request.DesiredLbs}, nil
}

// installWireguardVPN install wireguard VPN for all nodes in the infrastructure.
func installWireguardVPN(clusterID string, vpnInfo *VPNInfo, spawnProcessLimit chan struct{}) error {
	// Directory where files (required by Ansible) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, commonUtils.CreateHash(commonUtils.HashLength)))
	if err := commonUtils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := assignPrivateIPs(getAllNodepools(vpnInfo.NodepoolsInfos), vpnInfo.ClusterNetwork); err != nil {
		return fmt.Errorf("error while setting the private IPs for %s : %w", clusterDirectory, err)
	}

	if err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory,
		// Value of Ansible template parameters.
		AllNodesInventoryData{
			NodepoolsInfo: vpnInfo.NodepoolsInfos,
		},
	); err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	for _, nodepoolInfo := range vpnInfo.NodepoolsInfos {
		if err := commonUtils.CreateKeysForDynamicNodePools(nodepoolInfo.Nodepools.Dynamic, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
		}
		if err := commonUtils.CreateKeysForStaticNodepools(nodepoolInfo.Nodepools.Static, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
		}
	}
	ansible := utils.Ansible{
		Playbook:          wireguardPlaybookFilePath,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: spawnProcessLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("VPN - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}

// getAllNodepools flattens []*DesiredClusterNodepoolsInfo to []*pb.NodePool.
// Returns a slice of all the nodepools.
func getAllNodepools(nodepoolsInfo []*NodepoolsInfo) []*spec.NodePool {
	var nodepools []*spec.NodePool
	for _, nodepoolInfo := range nodepoolsInfo {
		nodepools = append(nodepools, nodepoolInfo.Nodepools.Dynamic...)
		nodepools = append(nodepools, nodepoolInfo.Nodepools.Static...)
	}

	return nodepools
}

// assignPrivateIPs will assign private IP addresses from the specified cluster network CIDR to all the nodes.
// Nodes which already have private IPs assigned will be ignored.
func assignPrivateIPs(nodepools []*spec.NodePool, cidr string) error {
	network, err := netip.ParsePrefix(cidr)
	if err != nil {
		return err
	}

	var (
		assignedPrivateIPs    = make(map[string]struct{})
		nodesWithoutPrivateIP []*spec.Node
	)

	// Construct nodesWithoutPrivateIP.
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			if node.Private != "" {
				assignedPrivateIPs[node.Private] = struct{}{}
			} else {
				nodesWithoutPrivateIP = append(nodesWithoutPrivateIP, node)
			}
		}
	}

	for address := network.Addr().Next(); network.Contains(address) && len(nodesWithoutPrivateIP) > 0; address = address.Next() {
		// If private IP is already assigned to some node
		// then skip that IP.
		if _, ok := assignedPrivateIPs[address.String()]; ok {
			continue
		}

		// Otherwise assign it to the node.
		nodesWithoutPrivateIP[0].Private = address.String()
		nodesWithoutPrivateIP = nodesWithoutPrivateIP[1:]
	}

	if len(nodesWithoutPrivateIP) > 0 {
		return errors.New("failed to assign private IPs to all nodes")
	}

	return nil
}
