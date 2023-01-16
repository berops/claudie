package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/ansibler/server/ansible"
	"golang.org/x/sync/errgroup"
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
				err := setPrivateAddresses(groupNodepool(vpnInfo.NodepoolInfo), vpnInfo.Network)
				if err != nil {
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
				err = generateInventoryFile(nodesInventoryFileTpl, directory, AllNodesInventoryData{NodepoolInfos: vpnInfo.NodepoolInfo})
				if err != nil {
					return fmt.Errorf("error while creating inventory file for %s : %w", directory, err)
				}
				//start ansible playbook
				ansible := ansible.Ansible{Playbook: wireguardPlaybook, Inventory: inventoryFile, Directory: directory}
				err = ansible.RunAnsiblePlaybook(fmt.Sprintf("VPN - %s", directory))
				if err != nil {
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
	err := errGroup.Wait()
	if err != nil {
		return fmt.Errorf("error while installing VPN : %w", err)
	}
	return nil
}

// setPrivateAddresses will assign private ip addresses from network range
// it will ignore the addresses which were previously assigned to an existing nodes
func setPrivateAddresses(nodepools []*pb.NodePool, network string) error {
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR %s : %w", network, err)
	}
	var nodesToAssignIP []*pb.Node

	// initialize slice of possible last octet
	lastOctets := make([]byte, 255)
	var i byte
	for i = 0; i < 255; i++ {
		lastOctets[i] = i + 1
	}

	ip := ipNet.IP
	ip = ip.To4()
	// remove last octets from slice of all possible ones
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			// If address already assigned, skip
			if node.Private != "" {
				lastOctet := strings.Split(node.Private, ".")[3]
				lastOctetInt, err := strconv.Atoi(lastOctet)
				if err != nil {
					return fmt.Errorf("failed to parse last octet %s of IP %s from node %s : %w", lastOctet, node.Private, node.Name, err)
				}
				lastOctets = remove(lastOctets, byte(lastOctetInt))
				continue
			}
			// add to slice of nodes which need new Ip assigned
			nodesToAssignIP = append(nodesToAssignIP, node)
		}
	}

	// assign new private IP to new nodes
	for i, node := range nodesToAssignIP {
		ip[3] = lastOctets[i]
		node.Private = ip.String()
	}
	return nil
}

// remove removes a value from the given slice
// if value not found in slice, returns original slice
func remove(slice []byte, value byte) []byte {
	for idx, v := range slice {
		if v == value {
			return append(slice[:idx], slice[idx+1:]...)
		}
	}
	return slice
}

// groupNodepool takes a nodepools from slice of Nodepool infos and return slice of all pb.Nodepools
func groupNodepool(nodepoolInfo []*NodepoolInfo) []*pb.NodePool {
	var nodepools []*pb.NodePool
	for _, np := range nodepoolInfo {
		nodepools = append(nodepools, np.Nodepools...)
	}
	return nodepools
}
