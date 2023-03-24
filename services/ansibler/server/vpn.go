package main

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/ansible"
)

const (
	wireguardPlaybook = "../../ansible-playbooks/wireguard.yml"
)

type VPNInfo struct {
	// Network is the network range
	Network string
	// NodepoolInfo are the pools used for the VPN
	NodepoolInfo []*NodepoolInfo
}

// installWireguardVPN sets up wireguard vpn for the nodepools
func installWireguardVPN(clusterID string, info *VPNInfo) error {
	directory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, utils.CreateHash(utils.HashLength)))

	if err := assignPrivateAddresses(groupNodepool(info.NodepoolInfo), info.Network); err != nil {
		return fmt.Errorf("error while setting the private IPs for %s : %w", directory, err)
	}

	if err := utils.CreateDirectory(directory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", directory, err)
	}

	for _, nodepoolInfo := range info.NodepoolInfo {
		if err := utils.CreateKeyFile(nodepoolInfo.PrivateKey, directory, fmt.Sprintf("%s.%s", nodepoolInfo.ID, privateKeyExt)); err != nil {
			return fmt.Errorf("failed to create key file for %s : %w", nodepoolInfo.ID, err)
		}
	}

	if err := generateInventoryFile(nodesInventoryFileTpl, directory, AllNodesInventoryData{NodepoolInfos: info.NodepoolInfo}); err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", directory, err)
	}

	ansible := ansible.Ansible{Playbook: wireguardPlaybook, Inventory: inventoryFile, Directory: directory}
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("VPN - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", directory, err)
	}

	return os.RemoveAll(directory)
}

// assignPrivateAddresses will assign private IPs addresses from the specified network range
// ignoring addresses that were previously assigned to existing nodes
func assignPrivateAddresses(nodepools []*pb.NodePool, cidr string) error {
	network, err := netip.ParsePrefix(cidr)
	if err != nil {
		return err
	}

	assignedIPs := make(map[string]struct{})
	var nodes []*pb.Node

	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			if node.Private != "" {
				assignedIPs[node.Private] = struct{}{}
			} else {
				nodes = append(nodes, node)
			}
		}
	}

	for address := network.Addr().Next(); network.Contains(address) && len(nodes) > 0; address = address.Next() {
		if _, ok := assignedIPs[address.String()]; ok {
			continue
		}
		nodes[len(nodes)-1].Private = address.String()
		nodes = nodes[:len(nodes)-1]
	}

	if len(nodes) > 0 {
		return errors.New("failed to assign private IPs to all nodes")
	}

	return nil
}

// groupNodepool takes a nodepools from slice of Nodepool infos and return slice of all pb.Nodepools
func groupNodepool(nodepoolInfo []*NodepoolInfo) []*pb.NodePool {
	var nodepools []*pb.NodePool
	for _, np := range nodepoolInfo {
		nodepools = append(nodepools, np.Nodepools...)
	}
	return nodepools
}
