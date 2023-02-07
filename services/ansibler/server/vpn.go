package main

import (
	"fmt"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/ansibler/server/ansible"
	"golang.org/x/sync/errgroup"
	"net/netip"
	"os"
	"path/filepath"
)

const (
	wireguardPlaybook = "../../ansible-playbooks/wireguard.yml"
)

type VPNInfo struct {
	Network      string          //network range
	NodepoolInfo []*NodepoolInfo //nodepools which will be inside the VPN
}

// installWireguardVPN takes a map of [k8sClusterName]*VPNInfo and sets up the wireguard vpn
// return error if not successful, nil otherwise
func installWireguardVPN(vpnNodepools map[string]*VPNInfo) error {
	var errGroup errgroup.Group
	for k8sClusterName, vpnInfo := range vpnNodepools {
		directory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", k8sClusterName, utils.CreateHash(4)))
		func(vpnInfo *VPNInfo) {
			//concurrent vpn creation on cluster level
			errGroup.Go(func() error {
				if err := assignPrivateAddresses(groupNodepool(vpnInfo.NodepoolInfo), vpnInfo.Network); err != nil {
					return fmt.Errorf("error while setting the private IPs for %s : %w", directory, err)
				}
				//generate key files
				if _, err := os.Stat(directory); os.IsNotExist(err) {
					if err := os.MkdirAll(directory, os.ModePerm); err != nil {
						return fmt.Errorf("failed to create directory %s : %w", directory, err)
					}
				}
				for _, nodepoolInfo := range vpnInfo.NodepoolInfo {
					if err := utils.CreateKeyFile(nodepoolInfo.PrivateKey, directory, fmt.Sprintf("%s.%s", nodepoolInfo.ID, privateKeyExt)); err != nil {
						return fmt.Errorf("failed to create key file for %s : %w", nodepoolInfo.ID, err)
					}
				}
				//generate inventory
				if err := generateInventoryFile(nodesInventoryFileTpl, directory, AllNodesInventoryData{NodepoolInfos: vpnInfo.NodepoolInfo}); err != nil {
					return fmt.Errorf("error while creating inventory file for %s : %w", directory, err)
				}
				//start ansible playbook
				ansible := ansible.Ansible{Playbook: wireguardPlaybook, Inventory: inventoryFile, Directory: directory}
				if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("VPN - %s", directory)); err != nil {
					return fmt.Errorf("error while running ansible for %s : %w", directory, err)
				}
				//Clean up
				if err := os.RemoveAll(directory); err != nil {
					return fmt.Errorf("error while deleting directory %s : %w", directory, err)
				}
				return nil
			})
		}(vpnInfo)
	}

	return errGroup.Wait()
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

	for address := network.Addr(); network.Contains(address) && len(nodes) > 0; address = address.Next() {
		if _, ok := assignedIPs[address.String()]; ok {
			continue
		}
		nodes[len(nodes)-1].Private = address.String()
		nodes = nodes[:len(nodes)-1]
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
