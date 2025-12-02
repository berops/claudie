package service

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

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

const wireguardPlaybook = "../../ansible-playbooks/wireguard.yml"

type VPNInfo struct {
	ClusterNetwork string

	// NodepoolsInfos is a slice with each element of type *DesiredClusterNodepoolsInfo.
	// Each element corresponds to a cluster (either a Kubernetes cluster or attached LB clusters).
	NodepoolsInfos []*NodepoolsInfo
}

// InstallVPN install wiregaurd VPN across all of the Loadbalancer and kubernetes nodes.
func InstallVPN(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Installing VPN")

	var k8s *spec.K8SclusterV2
	var lbs []*spec.LBclusterV2

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = do.Create.K8S
		lbs = do.Create.LoadBalancers
	case *spec.TaskV2_Update:
		k8s = do.Update.State.K8S
		lbs = do.Update.State.LoadBalancers
	default:
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to install vpn, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	vi := VPNInfo{
		ClusterNetwork: k8s.Network,
		NodepoolsInfos: []*NodepoolsInfo{
			{
				Nodepools: utils.NodePools{
					Dynamic: nodepools.Dynamic(k8s.ClusterInfo.NodePools),
					Static:  nodepools.Static(k8s.ClusterInfo.NodePools),
				},
				ClusterID:      k8s.ClusterInfo.Id(),
				ClusterNetwork: k8s.Network,
			},
		},
	}

	for _, lb := range lbs {
		vi.NodepoolsInfos = append(vi.NodepoolsInfos, &NodepoolsInfo{
			Nodepools: utils.NodePools{
				Dynamic: nodepools.Dynamic(lb.ClusterInfo.NodePools),
				Static:  nodepools.Static(lb.ClusterInfo.NodePools),
			},
			ClusterID:      lb.ClusterInfo.Id(),
			ClusterNetwork: k8s.Network,
		})
	}

	if err := installWireguardVPN(k8s.ClusterInfo.Id(), &vi, processLimit); err != nil {
		logger.Err(err).Msg("Failed to install VPN")
		tracker.Diagnostics.Push(err)
		// as there could be partial results, fallthrough.
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Loadbalancers(lbs...)
	update.Commit()
}

// installWireguardVPN install wireguard VPN for all nodes in the infrastructure.
func installWireguardVPN(clusterID string, vpnInfo *VPNInfo, processLimit *semaphore.Weighted) error {
	// Directory where files (required by Ansible) will be generated.
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			log.Err(err).Msgf("error while deleting files in %s", clusterDirectory)
		}
	}()

	if err := assignPrivateIPs(getAllNodepools(vpnInfo.NodepoolsInfos), vpnInfo.ClusterNetwork); err != nil {
		return fmt.Errorf("error while setting the private IPs for %s : %w", clusterDirectory, err)
	}

	data := AllNodesInventoryData{
		NodepoolsInfo: vpnInfo.NodepoolsInfos,
	}
	if err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory, data); err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	for _, nodepoolInfo := range vpnInfo.NodepoolsInfos {
		if err := nodepools.DynamicGenerateKeys(nodepoolInfo.Nodepools.Dynamic, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
		}
		if err := nodepools.StaticGenerateKeys(nodepoolInfo.Nodepools.Static, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
		}
	}
	ansible := utils.Ansible{
		Playbook:          wireguardPlaybook,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("VPN - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	return nil
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
