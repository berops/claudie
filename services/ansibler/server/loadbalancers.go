package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

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

type APIEndpointChangeState string

const (
	// NoChange represents the 1st. case, no change is needed as LB is currently
	// attached and the desired spec contains no changes.
	NoChange APIEndpointChangeState = "no-change"

	// AttachingLoadBalancer represents 2nd. case, the cluster previously
	// didn't have a LB and the ports needed to communicate with the API server
	// were exposed. After attaching a LB to the existing cluster the ports
	// were closed and are no longer accessible, and thus we need to change the API endpoint.
	AttachingLoadBalancer APIEndpointChangeState = "attaching-load-balancer"

	// DetachingLoadBalancer represents 3rd. case, the cluster had an existing
	// LB attached but the new state removed the LB and thus the API endpoint
	// needs to be changed back to one of the control nodes of the cluster.
	DetachingLoadBalancer APIEndpointChangeState = "detaching-load-balancer"

	// EndpointRenamed represents the 4th. case, the cluster had an existing
	// LB attached and also keeps it but the endpoint has changed.
	EndpointRenamed APIEndpointChangeState = "endpoint-renamed"

	// RoleChangedToAPIServer represents the 5th case, the cluster had an existing
	// LB attached that didn't have a ApiServer role attach but the desired state does.
	RoleChangedToAPIServer APIEndpointChangeState = "role-changed-to-api-server"

	// RoleChangedFromAPIServer represents the 6th case, the cluster had an existing
	// LB attached that had an ApiServer role attached but the desired state doesn't.
	RoleChangedFromAPIServer APIEndpointChangeState = "role-changed-from-api-server"
)

type (
	// LBInfo wraps all Load-balancers and Nodepools used for a single k8s cluster.
	LBInfo struct {
		// LbClusters are Load-Balancers that share the targeted k8s cluster.
		LbClusters []*LBData
		// TargetK8sNodepool are all nodepools used by the targeted k8s cluster.
		TargetK8sNodepool []*pb.NodePool
		// TargetK8sNodepoolKey is the key used for the nodepools.
		TargetK8sNodepoolKey string
		// PreviousAPIEndpointLB holds the endpoint of the previous Load-Balancer endpoint
		// if there was any to be able to handle the endpoint change.
		PreviousAPIEndpointLB string
	}

	LBData struct {
		// CurrentLbCluster is the current spec.
		// A value of nil means that the current k8s cluster
		// didn't have a LB attached to it.
		CurrentLbCluster *pb.LBcluster

		// DesiredLbCluster is the desired spec.
		// A value of nil means that the targeted k8s cluster
		// will no longer use a LoadBalancer.
		DesiredLbCluster *pb.LBcluster
	}

	NginxPlaybookData struct {
		Loadbalancer string
	}

	ConfData struct {
		Roles []LBConfiguration
	}

	LBConfiguration struct {
		Role        *pb.Role
		TargetNodes []*pb.Node
	}
)

// APIEndpointState determines if the API endpoint should be updated with a new
// address, as otherwise communication with the cluster wouldn't be possible.
func (lb *LBData) APIEndpointState() APIEndpointChangeState {
	if lb.CurrentLbCluster == nil && lb.DesiredLbCluster == nil {
		return NoChange
	}

	if lb.CurrentLbCluster == nil && lb.DesiredLbCluster != nil {
		return AttachingLoadBalancer
	}

	if lb.CurrentLbCluster != nil && lb.DesiredLbCluster == nil {
		return DetachingLoadBalancer
	}

	if lb.CurrentLbCluster.Dns.Endpoint != lb.DesiredLbCluster.Dns.Endpoint {
		return EndpointRenamed
	}

	// check if role changed.
	isAPIServer := hasAPIServerRole(lb.CurrentLbCluster.Roles)
	if hasAPIServerRole(lb.DesiredLbCluster.Roles) && !isAPIServer {
		return RoleChangedToAPIServer
	}

	if isAPIServer && !hasAPIServerRole(lb.DesiredLbCluster.Roles) {
		return RoleChangedFromAPIServer
	}

	return NoChange
}

// tearDownLoadBalancers will correctly destroy load-balancers including correctly selecting the new ApiServer if present.
// If for a k8sCluster a new ApiServerLB is being attached instead of handling the apiEndpoint immediately it will be delayed and
// will send the data to the dataChan which will be used later for the SetupLoadbalancers function to bypass generating the
// certificates for the endpoint multiple times.
func teardownLoadBalancers(deleted map[string]*LBInfo, attached map[string]bool, oldAPIEndpoints *sync.Map) error {
	var k8sGroup errgroup.Group

	for k8sClusterName, deleted := range deleted {
		k8sDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", k8sClusterName, utils.CreateHash(4)))

		if err := generateK8sBaseFiles(k8sDirectory, deleted); err != nil {
			log.Error().Msgf("error while generating base directory for %s : %v", k8sClusterName, err)
			continue
		}

		func(deleted *LBInfo, k8sClusterName string) {
			k8sGroup.Go(func() error {
				apiServer := findCurrentAPILoadBalancer(deleted.LbClusters)

				// if there was a apiServer that is deleted, and we're attaching a new
				// api server for the k8s cluster we store the old endpoint that will
				// be used later in the SetUpLoadbalancers function.
				if apiServer != nil && attached[k8sClusterName] {
					oldAPIEndpoints.Store(k8sClusterName, apiServer.CurrentLbCluster.Dns.Endpoint)
				} else {
					if err := handleAPIEndpointChange(apiServer, deleted, k8sDirectory); err != nil {
						return err
					}
				}

				return os.RemoveAll(k8sDirectory)
			})
		}(deleted, k8sClusterName)
	}

	return k8sGroup.Wait()
}

// setUpLoadbalancers will set up and verify the loadbalancer configuration including DNS
// returns error if not successful, nil otherwise
func setUpLoadbalancers(lbInfos map[string]*LBInfo) error {
	var errGroupK8s errgroup.Group
	//iterate through k8s clusters LBs
	for k8sClusterName, lbInfo := range lbInfos {
		//set up all lbs for the k8s cluster, since single k8s can have multiple LBs
		k8sDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", k8sClusterName, utils.CreateHash(4)))

		//generate inventory for all LBs with k8s nodes
		if err := generateK8sBaseFiles(k8sDirectory, lbInfo); err != nil {
			log.Error().Msgf("error while generating base directory for %s : %v", k8sClusterName, err)
			//continue with the next k8s cluster LBs
			continue
		}

		func(lbInfo *LBInfo, k8sClusterName string) {
			//process LB clusters for single K8s cluster
			errGroupK8s.Go(func() error {
				var errGroupLB errgroup.Group
				var apiServerLB *LBData

				//iterate over all LBs for a single k8s cluster
				for _, lb := range lbInfo.LbClusters {
					directory := filepath.Join(k8sDirectory, fmt.Sprintf("%s-%s", lb.DesiredLbCluster.ClusterInfo.Name, lb.DesiredLbCluster.ClusterInfo.Hash))
					func(lb *LBData) {
						errGroupLB.Go(func() error {
							log.Info().Msgf("Setting up the LB %s", directory)
							return setUpNginx(lb.DesiredLbCluster, lbInfo.TargetK8sNodepool, directory)
						})

						if hasAPIServerRole(lb.DesiredLbCluster.Roles) {
							apiServerLB = lb
						}
					}(lb)
				}

				if err := errGroupLB.Wait(); err != nil {
					return fmt.Errorf("error while setting up the loadbalancers for k8s cluster %s : %w", k8sClusterName, err)
				}

				// if we didn't found any ApiServerLB among the desired state LBs
				// it's possible that we've changed the role from an API server to
				// some other role. Which won't be caught by the above check, and we
				// have to do an additional check for the ApiServerLB in the current state.
				if apiServerLB == nil {
					apiServerLB = findCurrentAPILoadBalancer(lbInfo.LbClusters)
				}

				if err := handleAPIEndpointChange(apiServerLB, lbInfo, k8sDirectory); err != nil {
					return fmt.Errorf("failed to find a candidate for the Api Server: %w", err)
				}

				return os.RemoveAll(k8sDirectory)
			})
		}(lbInfo, k8sClusterName)
	}

	return errGroupK8s.Wait()
}

func handleAPIEndpointChange(apiServer *LBData, k8sCluster *LBInfo, k8sDirectory string) error {
	if apiServer == nil {
		// if there is no ApiSever LB that means that the ports 6443 are exposed
		// on the nodes, and thus we don't need to anything.
		return nil
	}

	var oldEndpoint string
	var newEndpoint string

	switch apiServer.APIEndpointState() {
	case NoChange:
		return nil
	case EndpointRenamed:
		oldEndpoint = apiServer.CurrentLbCluster.Dns.Endpoint
		newEndpoint = apiServer.DesiredLbCluster.Dns.Endpoint
	case RoleChangedFromAPIServer:
		// choose one of the control nodes as the api endpoint.
		node, err := findAPIEndpointNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return err
		}

		oldEndpoint = apiServer.CurrentLbCluster.Dns.Endpoint
		newEndpoint = node.Public
	case RoleChangedToAPIServer:
		newEndpoint = apiServer.DesiredLbCluster.Dns.Endpoint

		// 1st check if there was any APISERVER-LB previously attached to the k8scluster.
		if k8sCluster.PreviousAPIEndpointLB != "" {
			oldEndpoint = k8sCluster.PreviousAPIEndpointLB
			break
		}

		// 2nd check if any other LB was previously an ApiServer.
		if oldAPIServer := findCurrentAPILoadBalancer(k8sCluster.LbClusters); oldAPIServer != nil {
			oldEndpoint = oldAPIServer.CurrentLbCluster.Dns.Endpoint
			break
		}

		// 3rd pick the control node as the previous ApiServer.
		node, err := findAPIEndpointNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return fmt.Errorf("failed to find ApiEndpoint k8s node, couldn't update Api server endpoint")
		}

		oldEndpoint = node.Public
	case AttachingLoadBalancer:
		newEndpoint = apiServer.DesiredLbCluster.Dns.Endpoint

		// Try to find if one of the control nodes was the old ApiServer endpoint.
		node, err := findAPIEndpointNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			// If no Node has type ApiEndpoint this means that the cluster
			// wasn't build yet (i.e. it's the first time the manifest goes
			// through the workflow), thus we don't need to change the api endpoint.
			return nil
		}

		// We now know that it's not a first run, so before we use the node as the old APIServer
		// endpoint we check a few other possibilities.

		// 1st. check if there was any APIServer-LB previously attached to the k8scluster
		if k8sCluster.PreviousAPIEndpointLB != "" {
			oldEndpoint = k8sCluster.PreviousAPIEndpointLB
			break
		}

		// 2nd check if any other LB was previously an APIServer.
		if oldAPIServer := findCurrentAPILoadBalancer(k8sCluster.LbClusters); oldAPIServer != nil {
			oldEndpoint = oldAPIServer.CurrentLbCluster.Dns.Endpoint
			break
		}

		oldEndpoint = node.Public
	case DetachingLoadBalancer:
		// Choose one of the control nodes as the API endpoint.
		node, err := findAPIEndpointNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return err
		}

		oldEndpoint = apiServer.CurrentLbCluster.Dns.Endpoint
		newEndpoint = node.Public
	}

	lbCluster := apiServer.DesiredLbCluster
	if lbCluster == nil {
		lbCluster = apiServer.CurrentLbCluster
	}

	log.Info().Msgf("Changing the API endpoint for the cluster %s", lbCluster.ClusterInfo.Name)

	if err := changeAPIEndpoint(lbCluster.ClusterInfo.Name, oldEndpoint, newEndpoint, k8sDirectory); err != nil {
		return fmt.Errorf("error while changing the endpoint for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

// findAPIEndpointNode searches the NodePools for a Node with type ApiEndpoint.
// If no such node is found an error is returned.
func findAPIEndpointNode(nodepools []*pb.NodePool) (*pb.Node, error) {
	for _, nodePool := range nodepools {
		for _, node := range nodePool.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint {
				return node, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_apiEndpoint.String())
}

// findCurrentAPILoadBalancers finds the current Load-Balancer for the API server
func findCurrentAPILoadBalancer(lbs []*LBData) *LBData {
	for _, lb := range lbs {
		if lb.CurrentLbCluster != nil {
			if hasAPIServerRole(lb.CurrentLbCluster.Roles) {
				return lb
			}
		}
	}

	return nil
}

// hasAPIServerRole checks if there is an API server role.
func hasAPIServerRole(roles []*pb.Role) bool {
	for _, role := range roles {
		if role.RoleType == pb.RoleType_ApiServer {
			return true
		}
	}

	return false
}

// changeAPIEndpoint will change kubeadm configuration to include new EP
func changeAPIEndpoint(clusterName, oldEndpoint, newEndpoint, directory string) error {
	log.Info().Msgf("New endpoint is %s", newEndpoint)

	ansible := ansible.Ansible{
		Playbook:  apiChangePlaybook,
		Inventory: inventoryFile,
		Flags:     fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s\"", newEndpoint, oldEndpoint),
		Directory: directory,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", clusterName)); err != nil {
		return fmt.Errorf("error while running ansible: %w ", err)
	}

	return nil
}

// setUpNginx sets up the nginx loadbalancer based on the input manifest specification
// return error if not successful, nil otherwise
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

// splitNodesByType returns two slices of *pb.Node, one for control nodes and one for compute
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

// generateK8sBaseFiles generates the base loadbalancer files, like inventory, keys, etc.
// return error if not successful, nil otherwise
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
		if lb.DesiredLbCluster != nil {
			lbSlice = append(lbSlice, lb.DesiredLbCluster)
		}
	}
	//generate inventory
	err := generateInventoryFile(lbInventoryFile, k8sDirectory, LbInventoryData{K8sNodepools: lbInfo.TargetK8sNodepool, LBClusters: lbSlice})
	if err != nil {
		return fmt.Errorf("error while generating inventory file for %s : %w", k8sDirectory, err)
	}
	return nil
}

// assignTarget returns a target nodes for pb.Target
// if no target matches the pb.Target enum, returns nil
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
