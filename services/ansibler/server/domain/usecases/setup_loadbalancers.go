package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/berops/claudie/internal/templateUtils"
	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
)

const (
	nginxPlaybookName         = "nginx.yml"
	nginxConfTemplateFileName = "conf.gotpl"
)

type (
	LBClusterRolesInfo struct {
		Role        *pb.Role
		TargetNodes []*pb.Node
	}

	NginxConfParameters struct {
		Roles []LBClusterRolesInfo
	}
)

// SetUpLoadbalancers sets up the loadbalancers along with DNS and verifies their configuration
func (u *Usecases) SetUpLoadbalancers(request *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	logger := commonUtils.CreateLoggerWithProjectAndClusterName(request.ProjectName, commonUtils.GetClusterID(request.Desired.ClusterInfo))
	logger.Info().Msgf("Setting up the loadbalancers")

	// TODO: implement

	logger.Info().Msgf("Loadbalancers were successfully set up")
	return &pb.SetUpLBResponse{}, nil
}

// setUpLoadbalancers sets up and verifies the loadbalancer configuration (including DNS).
func setUpLoadbalancers(clusterName string, lbClustersInfo *utils.LBClustersInfo, logger zerolog.Logger) error {
	outputDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", clusterName, commonUtils.CreateHash(commonUtils.HashLength)))

	if err := generateLBInventoryFile(outputDirectory, lbClustersInfo); err != nil {
		return fmt.Errorf("error encountered while generating base files for %s", clusterName)
	}

	err := commonUtils.ConcurrentExec(lbClustersInfo.LbClusters,
		func(lbCluster *utils.LBClusterData) error {
			var (
				loggerPrefix = "LB-cluster"
				lbClusterId  = commonUtils.GetClusterID(lbCluster.DesiredLbCluster.ClusterInfo)
			)

			logger.Info().Str(loggerPrefix, lbClusterId).Msg("Setting up the loadbalancer cluster")

			// Create the directory where files will be generated
			outputDirectory = filepath.Join(outputDirectory, lbClusterId)
			if err := commonUtils.CreateDirectory(outputDirectory); err != nil {
				return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
			}

			// Generate SSH key
			if err := commonUtils.CreateKeyFile(lbCluster.DesiredLbCluster.ClusterInfo.PrivateKey, outputDirectory, fmt.Sprintf("key.%s", sshPrivateKeyFileExtension)); err != nil {
				return fmt.Errorf("failed to create key file for %s : %w", lbCluster.DesiredLbCluster.ClusterInfo.Name, err)
			}

			if err := setUpNodeExporter(lbCluster.DesiredLbCluster, outputDirectory); err != nil {
				return err
			}

			if err := setUpNginx(lbCluster.DesiredLbCluster, lbClustersInfo.TargetK8sNodepool, outputDirectory); err != nil {
				return err
			}

			logger.Info().Str(loggerPrefix, lbClusterId).Msg("Loadbalancer cluster successfully set up")
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("error while setting up the loadbalancers for cluster %s : %w", clusterName, err)
	}

	return os.RemoveAll(outputDirectory)
}

func generateLBInventoryFile(outputDirectory string, lbClustersInfo *utils.LBClustersInfo) error {
	// Create the directory where files will be generated
	if err := commonUtils.CreateDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
	}

	// Generate SSH key which will be used by Ansible.
	if err := commonUtils.CreateKeyFile(lbClustersInfo.TargetK8sNodepoolKey, outputDirectory, "k8s.pem"); err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}

	var lbClusters []*pb.LBcluster
	for _, item := range lbClustersInfo.LbClusters {
		if item.DesiredLbCluster != nil {
			lbClusters = append(lbClusters, item.DesiredLbCluster)
		}
	}

	// Generate Ansible inventory file.
	err := utils.GenerateInventoryFile(lbInventoryFileName, outputDirectory,
		// Value of Ansible template parameters
		LbInventoryData{
			K8sNodepools: lbClustersInfo.TargetK8sNodepool,
			LBClusters:   lbClusters,
			ClusterID:    lbClustersInfo.ClusterID,
		},
	)
	if err != nil {
		return fmt.Errorf("error while generating inventory file for %s : %w", outputDirectory, err)
	}

	return nil
}

// setUpNginx sets up the nginx loadbalancer based on the input manifest specification.
// Return error if not successful, nil otherwise
func setUpNginx(lbCluster *pb.LBcluster, targetK8sNodepool []*pb.NodePool, outputDirectory string) error {
	targetControlNodes, targetComputeNodes := splitNodesByType(targetK8sNodepool)

	// construct []LBClusterRolesInfo for the given LB cluster
	var lbClusterRolesInfo []LBClusterRolesInfo
	for _, role := range lbCluster.Roles {
		target := assignTarget(targetControlNodes, targetComputeNodes, role.Target)

		if target == nil {
			return fmt.Errorf("target %v did not specify any nodes", role.Target)
		}
		lbClusterRolesInfo = append(lbClusterRolesInfo, LBClusterRolesInfo{Role: role, TargetNodes: target})
	}

	// Generate the nginx config file
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.AnsiblerTemplates}
	nginxConfTemplate, err := templateLoader.LoadTemplate(nginxConfTemplateFileName)
	if err != nil {
		return fmt.Errorf("error while loading %s template for %w", nginxConfTemplateFileName, err)
	}
	templateUtils.Templates{Directory: outputDirectory}.
		Generate(nginxConfTemplate, "lb.conf", NginxConfParameters{Roles: lbClusterRolesInfo})
	if err != nil {
		return fmt.Errorf("error while generating lb.conf for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	ansible := utils.Ansible{
		Playbook:  nginxPlaybookName,
		Inventory: filepath.Join("..", inventoryFileName),
		Directory: outputDirectory,
	}
	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash))
	if err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

// splitNodesByType returns two slices of *pb.Node, one for control nodes and one for compute nodes.
func splitNodesByType(nodepools []*pb.NodePool) (controlNodes, computeNodes []*pb.Node) {
	for _, nodepools := range nodepools {
		for _, node := range nodepools.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint || node.NodeType == pb.NodeType_master {
				controlNodes = append(controlNodes, node)
			} else {
				computeNodes = append(computeNodes, node)
			}
		}
	}

	return controlNodes, computeNodes
}

// assignTarget returns a target nodes for pb.Target.
// If no target matches the pb.Target enum, returns nil
func assignTarget(targetControlNodes, targetComputeNodes []*pb.Node, target pb.Target) (targetNodes []*pb.Node) {
	if target == pb.Target_k8sAllNodes {
		targetNodes = append(targetControlNodes, targetComputeNodes...)
	} else if target == pb.Target_k8sControlPlane {
		targetNodes = targetControlNodes
	} else if target == pb.Target_k8sComputePlane {
		targetNodes = targetComputeNodes
	}

	return targetNodes
}
