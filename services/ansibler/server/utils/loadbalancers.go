package utils

import (
	"fmt"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
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
		Dynamic []*spec.NodePool
		Static  []*spec.NodePool
	}

	// LBClustersInfo wraps all Load-balancers and Nodepools used for a single K8s cluster.
	LBClustersInfo struct {
		// LbClusters are Load-Balancers that share the targeted k8s cluster.
		Lbs []*spec.LBcluster
		// TargetK8sNodepool are all nodepools used by the targeted k8s cluster.
		TargetK8sNodepool []*spec.NodePool
		// ClusterID contains the ClusterName-Hash- prefix of the kubernetes cluster
		ClusterID string
	}

	LBClusterRolesInfo struct {
		Role        *spec.Role
		TargetNodes []*spec.Node
	}

	NodeExporterTemplateParams struct {
		LoadBalancer     string
		NodeExporterPort int
	}

	UninstallNginxParams struct {
		LoadBalancer string
	}

	EnvoyConfigTemplateParams struct {
		LoadBalancer string
		Roles        []LBClusterRolesInfo
	}

	EnvoyTemplateParams struct {
		LoadBalancer   string
		Role           string
		EnvoyAdminPort int32
	}
)

// GenerateLBBaseFiles generates the files like Ansible inventory file and SSH keys to be used by Ansible.
// Returns error if not successful, nil otherwise
func GenerateLBBaseFiles(outputDirectory string, lbClustersInfo *LBClustersInfo) error {
	// Create the directory where files will be generated
	if err := fileutils.CreateDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
	}

	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(lbClustersInfo.TargetK8sNodepool), outputDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(lbClustersInfo.TargetK8sNodepool), outputDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	var lbClusters []LBcluster
	for _, lb := range lbClustersInfo.Lbs {
		lbClusters = append(lbClusters, LBcluster{
			Name: lb.ClusterInfo.Name,
			Hash: lb.ClusterInfo.Hash,
			LBnodepools: NodePools{
				Dynamic: nodepools.Dynamic(lb.ClusterInfo.NodePools),
				Static:  nodepools.Static(lb.ClusterInfo.NodePools),
			},
		})
	}

	// Generate Ansible inventory file.
	err := GenerateInventoryFile(templates.LoadbalancerInventoryTemplate, outputDirectory,
		// Value of Ansible template parameters
		LBInventoryFileParameters{
			K8sNodepools: NodePools{
				Dynamic: nodepools.Dynamic(lbClustersInfo.TargetK8sNodepool),
				Static:  nodepools.Static(lbClustersInfo.TargetK8sNodepool),
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
