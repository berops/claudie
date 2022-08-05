package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/ansibler/server/ansible"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	lbInventoryFile   = "lb-inventory.goini"
	confFile          = "conf.gotpl"
	nginxPlaybookTpl  = "nginx.goyml"
	nginxPlaybook     = "nginx.yml"
	apiChangePlaybook = "../../ansible-playbooks/apiEndpointChange.yml"
)

type LBInfo struct {
	LbClusters           []*LBData
	TargetK8sNodepool    []*pb.NodePool
	TargetK8sNodepoolKey string
}

type LBData struct {
	CurrentDNS *pb.DNS
	LbCluster  *pb.LBcluster
}

type PlaybookData struct {
	Loadbalancer string
}

type ConfData struct {
	Roles []RoleNodes
}

type RoleNodes struct {
	Role        *pb.Role
	TargetNodes []*pb.Node
}

func setUpLoadbalancers(lbInfos map[string]*LBInfo) error {
	var errGroupK8s errgroup.Group
	for k8sClusterName, lbInfo := range lbInfos {
		func(lbInfo *LBInfo) {
			//set up all lbs for the k8s cluster, since single k8s can have multiple LBs
			k8sDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-lbs", k8sClusterName))
			//generate inventory for all LBs with k8s nodes
			generateK8sBaseFiles(k8sDirectory, lbInfo)
			errGroupK8s.Go(func() error {
				var errGroupLB errgroup.Group
				//set up the individual LB
				for _, lb := range lbInfo.LbClusters {
					directory := filepath.Join(k8sDirectory, lb.LbCluster.ClusterInfo.Name)
					func(directory, k8sDirectory string) {
						errGroupLB.Go(func() error {
							err := setUpNginx(lb.LbCluster, lbInfo.TargetK8sNodepool, directory)
							if err != nil {
								return err
							}
							//check for DNS change
							if utils.ChangedAPIEndpoint(lb.CurrentDNS, lb.LbCluster.Dns) {
								err = changedAPIEp(lb, k8sDirectory)
								if err != nil {
									return err
								}
							}
							return nil
						})
					}(directory, k8sDirectory)
				}
				err := errGroupLB.Wait()
				if err != nil {
					return err
				}
				//Clean up
				if err := os.RemoveAll(k8sDirectory); err != nil {
					return fmt.Errorf("error while deleting files: %v", err)
				}
				return nil
			})
		}(lbInfo)
	}
	err := errGroupK8s.Wait()
	if err != nil {
		return err
	}
	return nil
}

func changedAPIEp(lbInfo *LBData, directory string) error {
	log.Info().Msgf("Changing the API endpoint for the cluster %s", lbInfo.LbCluster.ClusterInfo.Name)
	log.Info().Msgf("New endpoint is %s", lbInfo.LbCluster.Dns.Endpoint)
	flag := fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s\"", lbInfo.LbCluster.Dns.Endpoint, lbInfo.CurrentDNS.Endpoint)
	ansible := ansible.Ansible{Playbook: apiChangePlaybook, Inventory: inventoryFile, Flags: flag, Directory: directory}
	err := ansible.RunAnsiblePlaybook("ENDPOINT CHANGE")
	if err != nil {
		return err
	}
	return nil
}

func setUpNginx(lb *pb.LBcluster, targetedNodepool []*pb.NodePool, directory string) error {
	//create key files for lb nodepools
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if err := os.MkdirAll(directory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}
	if err := utils.CreateKeyFile(lb.ClusterInfo.PrivateKey, directory, fmt.Sprintf("%s.%s", lb.ClusterInfo.Name, privateKeyExt)); err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	//prepare data for .conf
	templateLoader := utils.TemplateLoader{Directory: utils.AnsiblerTemplates}
	template := utils.Templates{Directory: directory}
	tpl, err := templateLoader.LoadTemplate(confFile)
	if err != nil {
		return err
	}
	//get control and compute nodes
	controlTarget, computeTarget := nodeSegregation(targetedNodepool)
	var lbRoles []RoleNodes
	for _, role := range lb.Roles {
		var target []*pb.Node
		if role.Target == pb.Target_k8sAllNodes {
			target = append(controlTarget, computeTarget...)
		} else if role.Target == pb.Target_k8sControlPlane {
			target = controlTarget
		} else if role.Target == pb.Target_k8sComputePlane {
			target = computeTarget
		}
		lbRoles = append(lbRoles, RoleNodes{Role: role, TargetNodes: target})
	}
	//create .conf file
	err = template.Generate(tpl, "lb.conf", ConfData{Roles: lbRoles})
	if err != nil {
		return err
	}
	tpl, err = templateLoader.LoadTemplate(nginxPlaybookTpl)
	if err != nil {
		return err
	}
	err = template.Generate(tpl, "nginx.yml", PlaybookData{Loadbalancer: lb.ClusterInfo.Name})
	if err != nil {
		return err
	}
	//run the playbook
	ansible := ansible.Ansible{Playbook: nginxPlaybook, Inventory: "../" + inventoryFile, Directory: directory}
	err = ansible.RunAnsiblePlaybook(lb.ClusterInfo.Name)
	if err != nil {
		return err
	}
	return nil
}

func nodeSegregation(nodepools []*pb.NodePool) (controlNodes, ComputeNodes []*pb.Node) {
	for _, nodepools := range nodepools {
		for _, node := range nodepools.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint || node.NodeType == pb.NodeType_master {
				controlNodes = append(controlNodes, node)
			} else {
				ComputeNodes = append(ComputeNodes, node)
			}
		}
	}
	return
}

func generateK8sBaseFiles(k8sDirectory string, lbInfo *LBInfo) error {
	if _, err := os.Stat(k8sDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(k8sDirectory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}
	if err := utils.CreateKeyFile(lbInfo.TargetK8sNodepoolKey, k8sDirectory, "k8s.pem"); err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	var lbSlice []*pb.LBcluster
	for _, lb := range lbInfo.LbClusters {
		lbSlice = append(lbSlice, lb.LbCluster)
	}
	//generate inventory
	err := generateInventoryFile(lbInventoryFile, k8sDirectory, LbInventoryData{K8sNodepools: lbInfo.TargetK8sNodepool, LBClusters: lbSlice})
	if err != nil {
		return err
	}
	return nil
}
