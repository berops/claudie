package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/ansibler/server/ansible"
	"github.com/Berops/platform/utils"
	"golang.org/x/sync/errgroup"
)

const (
	wireguardPlaybook = "../../ansible-playbooks/wireguard.yml"
)

type VPNInfo struct {
	Network      string          //network range
	NodepoolInfo []*NodepoolInfo //nodepools which will be inside the VPN
}

//installWireguardVPN takes a map of [k8sClusterName]*VPNInfo and sets up the wireguard vpn
//return error if not successful, nil otherwise
func installWireguardVPN(vpnNodepools map[string]*VPNInfo) error {
	var errGroup errgroup.Group
	for k8sClusterName, vpnInfo := range vpnNodepools {
		directory := filepath.Join(baseDirectory, outputDirectory, k8sClusterName)
		func(vpnInfo *VPNInfo) {
			//concurrent vpn creation on cluster level
			errGroup.Go(func() error {
				err := setPrivateAddresses(groupNodepool(vpnInfo.NodepoolInfo), vpnInfo.Network)
				if err != nil {
					return err
				}
				//generate key files
				if _, err := os.Stat(directory); os.IsNotExist(err) {
					if err := os.MkdirAll(directory, os.ModePerm); err != nil {
						return fmt.Errorf("failed to create dir: %v", err)
					}
				}
				for _, nodepoolInfo := range vpnInfo.NodepoolInfo {
					if err := utils.CreateKeyFile(nodepoolInfo.PrivateKey, directory, fmt.Sprintf("%s.%s", nodepoolInfo.ID, privateKeyExt)); err != nil {
						return fmt.Errorf("failed to create key file: %v", err)
					}
				}
				//generate inventory
				err = generateInventoryFile(nodesInventoryFile, directory, AllNodesInventoryData{NodepoolInfos: vpnInfo.NodepoolInfo})
				if err != nil {
					return err
				}
				//start ansible playbook
				ansible := ansible.Ansible{Playbook: wireguardPlaybook, Inventory: inventoryFile, Directory: directory}
				err = ansible.RunAnsiblePlaybook(directory)
				if err != nil {
					return err
				}
				//Clean up
				if err := os.RemoveAll(directory); err != nil {
					return fmt.Errorf("error while deleting files: %v", err)
				}
				return nil
			})
		}(vpnInfo)
	}
	err := errGroup.Wait()
	if err != nil {
		return err
	}
	return nil
}

// setPrivateAddresses will assign private ip addresses from network range
//it will ignore the addresses which were previously assigned to an existing nodes
func setPrivateAddresses(nodepools []*pb.NodePool, network string) error {
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR: %v", err)
	}
	var addressesToAssign []*pb.Node

	// initialize slice of possible last octet
	lastOctets := make([]byte, 255)
	var i byte
	for i = 0; i < 255; i++ {
		lastOctets[i] = i + 1
	}

	ip := ipNet.IP
	ip = ip.To4()
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			// If address already assigned, skip
			if node.Private != "" {
				lastOctet := strings.Split(node.Private, ".")[3]
				lastOctetInt, _ := strconv.Atoi(lastOctet)
				lastOctets = remove(lastOctets, byte(lastOctetInt))
				continue
			}
			addressesToAssign = append(addressesToAssign, node)
		}
	}

	var temp int
	for _, address := range addressesToAssign {
		ip[3] = lastOctets[temp]
		address.Private = ip.String()
		temp++
	}
	return nil
}

//remove removes a value from the given slice
//if value not found in slice, returns original slice
func remove(slice []byte, value byte) []byte {
	for idx, v := range slice {
		if v == value {
			return append(slice[:idx], slice[idx+1:]...)
		}
	}
	return slice
}

//groupNodepool takes a nodepools from slice of Nodepool infos and return slice of all pb.Nodepools
func groupNodepool(nodepoolInfo []*NodepoolInfo) []*pb.NodePool {
	var nodepools []*pb.NodePool
	for _, np := range nodepoolInfo {
		nodepools = append(nodepools, np.Nodepools...)
	}
	return nodepools
}
