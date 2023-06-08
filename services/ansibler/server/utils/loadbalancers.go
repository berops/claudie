package utils

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/templates"
)

/*
Loadbalancers are set up via ansible playbooks

The layout of the files/directories for a single k8s cluster loadbalancers is:

clusters/
└── k8s-cluster-1/
	│
	├── lb-cluster-1/
	│	├── key.pem
	│	├── lb.conf
	│	└── nginx.yml
	│
	├── lb-cluster-2/
	│	├── key.pem
	│	├── lb.conf
	│	└── nginx.yml
	├── k8s.pem
	└── inventory.ini
*/

type APIEndpointChangeState string

const (
	// NoChange represents the 1st case - no change is needed as the LB cluster is currently
	// attached and the desired spec contains no changes.
	NoChange APIEndpointChangeState = "no-change"

	// AttachingLoadBalancer represents 2nd case - the K8s cluster previously
	// didn't have an LB cluster attached and the ports needed to communicate with the API server
	// were exposed. After attaching an LB cluster to the existing K8s cluster the ports
	// were closed and are no longer accessible, and thus we need to change the API endpoint.
	AttachingLoadBalancer APIEndpointChangeState = "attaching-load-balancer"

	// DetachingLoadBalancer represents 3rd. case - the K8s cluster had an existing
	// LB cluster attached but the new state removed the LB cluster and thus the API endpoint
	// needs to be changed back to one of the control nodes of the cluster.
	DetachingLoadBalancer APIEndpointChangeState = "detaching-load-balancer"

	// EndpointRenamed represents the 4th. case - the K8s cluster has an existing
	// LB cluster attached and also keeps it but the endpoint has changed in the desired state.
	EndpointRenamed APIEndpointChangeState = "endpoint-renamed"

	// RoleChangedToAPIServer represents the 5th case - the K8s cluster has an existing
	// LB cluster attached that didn't have a ApiServer role attached but the desired state does.
	RoleChangedToAPIServer APIEndpointChangeState = "role-changed-to-api-server"

	// RoleChangedFromAPIServer represents the 6th case - the K8s cluster has an existing
	// LB cluster attached that had an ApiServer role attached but the desired state doesn't.
	RoleChangedFromAPIServer APIEndpointChangeState = "role-changed-from-api-server"
)

type (
	LBInventoryFileParameters struct {
		K8sNodepools NodePools
		LBClusters   []LBcluster
		ClusterID    string
	}

	LBcluster struct {
		Name        string
		Hash        string
		LBnodepools NodePools
	}

	NodePools struct {
		Dynamic []*pb.NodePool
		Static  []*pb.NodePool
	}

	// LBClustersInfo wraps all Load-balancers and Nodepools used for a single K8s cluster.
	LBClustersInfo struct {
		// LbClusters are Load-Balancers that share the targeted k8s cluster.
		LbClusters []*LBClusterData
		// TargetK8sNodepool are all nodepools used by the targeted k8s cluster.
		TargetK8sNodepool []*pb.NodePool
		// TargetK8sNodepoolKey is the key used for the nodepools.
		TargetK8sNodepoolKey string
		// PreviousAPIEndpointLB holds the endpoint of the previous Load-Balancer endpoint
		// if there was any to be able to handle the endpoint change.
		PreviousAPIEndpointLB string
		// ClusterID contains the ClusterName-Hash- prefix of the kubernetes cluster
		ClusterID string
		// Indicates whether the manifest has no current state i.e. it's the first time it's being build.
		FirstRun bool
	}

	// LBClusterData holds details about the current and desired state of an LB cluster.
	LBClusterData struct {
		// CurrentLbCluster is the current spec of the LB Cluster.
		// A value of nil means that the LB cluster doesn't exist currently
		// and will be created in the future.
		CurrentLbCluster *pb.LBcluster

		// DesiredLbCluster is the desired spec of the LB Cluster.
		// A value of nil means that this LB cluster will be deleted in the future.
		DesiredLbCluster *pb.LBcluster
	}

	LBPlaybookParameters struct {
		Loadbalancer string
	}

	LBClusterRolesInfo struct {
		Role        *pb.Role
		TargetNodes []*pb.Node
	}

	NginxConfigTemplateParameters struct {
		Roles []LBClusterRolesInfo
	}
)

// APIEndpointState determines if the API endpoint should be updated with a new
// address, as otherwise communication with the cluster wouldn't be possible.
func (lb *LBClusterData) APIEndpointState() APIEndpointChangeState {
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
	isAPIServer := utils.HasAPIServerRole(lb.CurrentLbCluster.Roles)
	if utils.HasAPIServerRole(lb.DesiredLbCluster.Roles) && !isAPIServer {
		return RoleChangedToAPIServer
	}

	if isAPIServer && !utils.HasAPIServerRole(lb.DesiredLbCluster.Roles) {
		return RoleChangedFromAPIServer
	}

	return NoChange
}

// GenerateLBBaseFiles generates the files like Ansible inventory file and SSH keys to be used by Ansible.
// Returns error if not successful, nil otherwise
func GenerateLBBaseFiles(outputDirectory string, lbClustersInfo *LBClustersInfo) error {
	// Create the directory where files will be generated
	if err := utils.CreateDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
	}

	// Generate SSH key which will be used by Ansible.
	if err := utils.CreateKeyFile(lbClustersInfo.TargetK8sNodepoolKey, outputDirectory, "k8s.pem"); err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}

	var lbClusters []LBcluster
	for _, lbData := range lbClustersInfo.LbClusters {
		if lbData.DesiredLbCluster != nil {
			lbClusters = append(lbClusters, LBcluster{
				Name: lbData.DesiredLbCluster.ClusterInfo.Name,
				Hash: lbData.DesiredLbCluster.ClusterInfo.Hash,
				LBnodepools: NodePools{
					Dynamic: GetDynamicNodepools(lbData.DesiredLbCluster.ClusterInfo.NodePools),
					Static:  GetStaticNodepools(lbData.DesiredLbCluster.ClusterInfo.NodePools),
				},
			})
		}
	}

	// Generate Ansible inventory file.
	err := GenerateInventoryFile(templates.LoadbalancerInventoryTemplate, outputDirectory,
		// Value of Ansible template parameters
		LBInventoryFileParameters{
			K8sNodepools: NodePools{
				Dynamic: GetDynamicNodepools(lbClustersInfo.TargetK8sNodepool),
				Static:  GetStaticNodepools(lbClustersInfo.TargetK8sNodepool),
			},
			LBClusters: lbClusters,
			ClusterID:  lbClustersInfo.ClusterID,
		},
	)
	if err != nil {
		return fmt.Errorf("error while generating inventory file for %s : %w", outputDirectory, err)
	}

	return nil
}

func HandleAPIEndpointChange(apiServerTypeLBCluster *LBClusterData, k8sCluster *LBClustersInfo, outputDirectory string) error {
	// If there is no ApiSever type LB cluster, that means that the ports 6443 are exposed
	// on one of the control nodes (which acts as the api endpoint).
	// Thus we don't need to do anything.
	if apiServerTypeLBCluster == nil {
		return nil
	}

	var oldEndpoint, newEndpoint string

	switch apiServerTypeLBCluster.APIEndpointState() {
	case NoChange:
		return nil

	case EndpointRenamed:
		oldEndpoint = apiServerTypeLBCluster.CurrentLbCluster.Dns.Endpoint
		newEndpoint = apiServerTypeLBCluster.DesiredLbCluster.Dns.Endpoint

	case RoleChangedFromAPIServer:
		oldEndpoint = apiServerTypeLBCluster.CurrentLbCluster.Dns.Endpoint

		// Find if any control node was acting as the Api endpoint in past.
		// If so, then we will reuse that control node as the Api endpoint.
		if node, err := utils.FindAPIEndpointNode(k8sCluster.TargetK8sNodepool); err == nil {
			newEndpoint = node.Public
			break
		}

		// Otherwise choose one of the control nodes as the api endpoint.
		node, err := utils.FindControlNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return err
		}
		node.NodeType = pb.NodeType_apiEndpoint
		newEndpoint = node.Public

	case RoleChangedToAPIServer:
		newEndpoint = apiServerTypeLBCluster.DesiredLbCluster.Dns.Endpoint

		// 1st - check if there was any Api server type LB cluster previously attached to the K8s cluster.
		if k8sCluster.PreviousAPIEndpointLB != "" {
			oldEndpoint = k8sCluster.PreviousAPIEndpointLB
			break
		}

		// 2nd - check if any other LB cluster was previously an ApiServer.
		if oldAPIServer := FindCurrentAPIServerTypeLBCluster(k8sCluster.LbClusters); oldAPIServer != nil {
			oldEndpoint = oldAPIServer.CurrentLbCluster.Dns.Endpoint
			break
		}

		// 3rd - pick the control node as the previous ApiServer.
		node, err := utils.FindAPIEndpointNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return fmt.Errorf("failed to find ApiEndpoint k8s node, couldn't update Api server endpoint")
		}
		oldEndpoint = node.Public

	case AttachingLoadBalancer:
		newEndpoint = apiServerTypeLBCluster.DesiredLbCluster.Dns.Endpoint

		// If it's the first time the manifest goes through the workflow,
		// we don't have an old Api endpoint. So nothing to do here, we will just return.
		if k8sCluster.FirstRun {
			return nil
		}

		// We know that it's not a first run, so before we use the node as the old APIServer
		// endpoint we check a few other possibilities.

		// 1st - check if there was any APIServer-LB previously attached to the k8scluster
		if k8sCluster.PreviousAPIEndpointLB != "" {
			oldEndpoint = k8sCluster.PreviousAPIEndpointLB
			break
		}

		// 2nd - check if any other LB was previously an APIServer.
		if oldAPIServer := FindCurrentAPIServerTypeLBCluster(k8sCluster.LbClusters); oldAPIServer != nil {
			oldEndpoint = oldAPIServer.CurrentLbCluster.Dns.Endpoint
			break
		}

		// 3rd - pick the control node as the previous ApiServer.
		node, err := utils.FindAPIEndpointNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return fmt.Errorf("failed to find APIEndpoint k8s node, couldn't update Api server endpoint")
		}
		node.NodeType = pb.NodeType_master // remove the Endpoint type from the node.
		oldEndpoint = node.Public

	case DetachingLoadBalancer:
		oldEndpoint = apiServerTypeLBCluster.CurrentLbCluster.Dns.Endpoint

		// 1st - find if any control node was an API server.
		if node, err := utils.FindAPIEndpointNode(k8sCluster.TargetK8sNodepool); err == nil {
			newEndpoint = node.Public
			break
		}

		// 2nd - choose one of the control nodes as the api endpoint.
		node, err := utils.FindControlNode(k8sCluster.TargetK8sNodepool)
		if err != nil {
			return err
		}
		node.NodeType = pb.NodeType_apiEndpoint
		newEndpoint = node.Public
	}

	lbCluster := apiServerTypeLBCluster.DesiredLbCluster
	if lbCluster == nil {
		lbCluster = apiServerTypeLBCluster.CurrentLbCluster
	}

	log.Debug().Str("LB-cluster", utils.GetClusterID(lbCluster.ClusterInfo)).Msgf("Changing the API endpoint from %s to %s", oldEndpoint, newEndpoint)
	if err := ChangeAPIEndpoint(lbCluster.ClusterInfo.Name, oldEndpoint, newEndpoint, outputDirectory); err != nil {
		return fmt.Errorf("error while changing the endpoint for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}
