package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/ansibler/server/ansible"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

/*
Loadbalancers are set up via ansible playbooks

The layout of the files/directories for a single k8s cluster loadbalancers is:

clusters/
└── k8s-cluster-1/
	├── lb-cluster-1/
	│	├── key.pem
	│	├── lb.conf
	│	└── nginx.yml
	├── lb-cluster-2/
	│	├── key.pem
	│	├── lb.conf
	│	└── nginx.yml
	├── k8s.pem
	└── inventory.ini
*/

const (
	lbInventoryFile   = "lb-inventory.goini"
	confFile          = "conf.gotpl"
	nginxPlaybookTpl  = "nginx.goyml"
	nginxPlaybook     = "nginx.yml"
	apiChangePlaybook = "../../ansible-playbooks/apiEndpointChange.yml"
)

type LBInfo struct {
	LbClusters           []*LBData      //cluster which have a same targeted k8s cluster
	TargetK8sNodepool    []*pb.NodePool //all nodepools of targeted k8s cluster
	TargetK8sNodepoolKey string         //nodepool key
}

type LBData struct {
	CurrentDNS *pb.DNS       //Current DNS spec
	LbCluster  *pb.LBcluster //Desired LB
}

type NginxPlaybookData struct {
	Loadbalancer string
}

type ConfData struct {
	Roles []LBConfiguration
}

type LBConfiguration struct {
	Role        *pb.Role
	TargetNodes []*pb.Node
}

//setUpLoadbalancers will set up and verify the loadbalancer configuration including DNS
//returns error if not successful, nil otherwise
func setUpLoadbalancers(lbInfos map[string]*LBInfo) error {
	var errGroupK8s errgroup.Group
	//iterate through k8s clusters LBs
	for k8sClusterName, lbInfo := range lbInfos {
		func(lbInfo *LBInfo, k8sClusterName string) {
			//set up all lbs for the k8s cluster, since single k8s can have multiple LBs
			k8sDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", k8sClusterName, utils.CreateHash(4)))
			//generate inventory for all LBs with k8s nodes
			err := generateK8sBaseFiles(k8sDirectory, lbInfo)
			if err != nil {
				log.Error().Msgf("error while generating base directory for %s : %w", k8sClusterName, err)
				return //continue with the next k8s cluster LBs
			}
			//process LB clusters for single K8s cluster
			errGroupK8s.Go(func() error {
				var errGroupLB errgroup.Group
				//iterate over all LBs for a single k8s cluster
				for _, lb := range lbInfo.LbClusters {
					directory := filepath.Join(k8sDirectory, fmt.Sprintf("%s-%s", lb.LbCluster.ClusterInfo.Name, lb.LbCluster.ClusterInfo.Hash))
					log.Info().Msgf("Setting up the LB %s", directory)
					func(directory, k8sDirectory string, lb *LBData) {
						//set up the individual LB
						errGroupLB.Go(func() error {
							//set up nginx
							err := setUpNginx(lb.LbCluster, lbInfo.TargetK8sNodepool, directory)
							if err != nil {
								return fmt.Errorf("error while setting up the nginx %s : %w", lb.LbCluster.ClusterInfo.Name, err)
							}
							//check for DNS change
							if utils.ChangedAPIEndpoint(lb.CurrentDNS, lb.LbCluster.Dns) {
								err = changedAPIEp(lb, k8sDirectory)
								if err != nil {
									return fmt.Errorf("error while changing the endpoint for %s : %w", lb.LbCluster.ClusterInfo.Name, err)
								}
							}
							return nil
						})
					}(directory, k8sDirectory, lb)
				}
				err := errGroupLB.Wait()
				if err != nil {
					return fmt.Errorf("error while setting up the loadbalancers for k8s cluster %s : %w", k8sClusterName, err)
				}
				//Clean up
				if err := os.RemoveAll(k8sDirectory); err != nil {
					return fmt.Errorf("error while deleting files: %w", err)
				}
				return nil
			})
		}(lbInfo, k8sClusterName)
	}
	err := errGroupK8s.Wait()
	if err != nil {
		return fmt.Errorf("error while setting up the loadbalancers : %w", err)
	}
	return nil
}

//changedAPIEp will change kubeadm configuration to include new EP
//return error if not successful, nil otherwise
func changedAPIEp(lbInfo *LBData, directory string) error {
	log.Info().Msgf("Changing the API endpoint for the cluster %s", lbInfo.LbCluster.ClusterInfo.Name)
	log.Info().Msgf("New endpoint is %s", lbInfo.LbCluster.Dns.Endpoint)
	flag := fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s\"", lbInfo.LbCluster.Dns.Endpoint, lbInfo.CurrentDNS.Endpoint)
	ansible := ansible.Ansible{Playbook: apiChangePlaybook, Inventory: inventoryFile, Flags: flag, Directory: directory}
	err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", lbInfo.LbCluster.ClusterInfo.Name))
	if err != nil {
		return fmt.Errorf("error while running ansible for EP change for %s : %w ", lbInfo.LbCluster.ClusterInfo.Name, err)
	}
	return nil
}

//setUpNginx sets up the nginx loadbalancer based on the input manifest specification
//return error if not successful, nil otherwise
func setUpNginx(lb *pb.LBcluster, targetedNodepool []*pb.NodePool, directory string) error {
	//create key files for lb nodepools
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if err := os.MkdirAll(directory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s : %w", directory, err)
		}
	}
	if err := utils.CreateKeyFile(lb.ClusterInfo.PrivateKey, directory, fmt.Sprintf("key.%s", privateKeyExt)); err != nil {
		return fmt.Errorf("failed to create key file for %s : %w", lb.ClusterInfo.Name, err)
	}
	//prepare data for .conf
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.AnsiblerTemplates}
	template := templateUtils.Templates{Directory: directory}
	tpl, err := templateLoader.LoadTemplate(confFile)
	if err != nil {
		return fmt.Errorf("error while loading %s template for %w", confFile, err)
	}
	//get control and compute nodes
	controlTarget, computeTarget := splitNodesByType(targetedNodepool)
	var lbRoles []LBConfiguration
	for _, role := range lb.Roles {
		target := assignTarget(controlTarget, computeTarget, role.Target)
		if target == nil {
			return fmt.Errorf("target %v did not specify any nodes", role.Target)
		}
		lbRoles = append(lbRoles, LBConfiguration{Role: role, TargetNodes: target})
	}
	//create .conf file
	err = template.Generate(tpl, "lb.conf", ConfData{Roles: lbRoles})
	if err != nil {
		return fmt.Errorf("error while generating lb.conf for %s : %w", lb.ClusterInfo.Name, err)
	}
	tpl, err = templateLoader.LoadTemplate(nginxPlaybookTpl)
	if err != nil {
		return fmt.Errorf("error while loading %s for %s : %w", nginxPlaybook, lb.ClusterInfo.Name, err)
	}
	err = template.Generate(tpl, "nginx.yml", NginxPlaybookData{Loadbalancer: lb.ClusterInfo.Name})
	if err != nil {
		return fmt.Errorf("error while generating %s for %s : %w", nginxPlaybook, lb.ClusterInfo.Name, err)
	}
	//run the playbook
	ansible := ansible.Ansible{Playbook: nginxPlaybook, Inventory: filepath.Join("..", inventoryFile), Directory: directory}
	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s", lb.ClusterInfo.Name))
	if err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lb.ClusterInfo.Name, err)
	}
	return nil
}

//splitNodesByType returns two slices of *pb.Node, one for control nodes and one for compute
func splitNodesByType(nodepools []*pb.NodePool) (controlNodes, ComputeNodes []*pb.Node) {
	for _, nodepools := range nodepools {
		for _, node := range nodepools.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint || node.NodeType == pb.NodeType_master {
				controlNodes = append(controlNodes, node)
			} else {
				ComputeNodes = append(ComputeNodes, node)
			}
		}
	}
	return controlNodes, ComputeNodes
}

//generateK8sBaseFiles generates the base loadbalancer files, like inventory, keys, etc.
//return error if not successful, nil otherwise
func generateK8sBaseFiles(k8sDirectory string, lbInfo *LBInfo) error {
	if _, err := os.Stat(k8sDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(k8sDirectory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %w", err)
		}
	}
	if err := utils.CreateKeyFile(lbInfo.TargetK8sNodepoolKey, k8sDirectory, "k8s.pem"); err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	var lbSlice []*pb.LBcluster
	for _, lb := range lbInfo.LbClusters {
		lbSlice = append(lbSlice, lb.LbCluster)
	}
	//generate inventory
	err := generateInventoryFile(lbInventoryFile, k8sDirectory, LbInventoryData{K8sNodepools: lbInfo.TargetK8sNodepool, LBClusters: lbSlice})
	if err != nil {
		return fmt.Errorf("error while generating inventory file for %s : %w", k8sDirectory, err)
	}
	return nil
}

//assignTarget returns a target nodes for pb.Target
//if no target matches the pb.Target enum, returns nil
func assignTarget(controlTarget, computeTarget []*pb.Node, target pb.Target) (targetNodes []*pb.Node) {
	if target == pb.Target_k8sAllNodes {
		targetNodes = append(controlTarget, computeTarget...)
	} else if target == pb.Target_k8sControlPlane {
		targetNodes = controlTarget
	} else if target == pb.Target_k8sComputePlane {
		targetNodes = computeTarget
	}
	return targetNodes
}
