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
	"github.com/berops/claudie/services/ansibler/templates"
)

const (
	// nodeExporterPlaybookFileName defines name for node exporter playbook.
	nodeExporterPlaybookFileName = "node-exporter.yml"
	// nodeExporterPlaybookFileName defines name for nginx playbook.
	nginxPlaybookName = "nginx.yml"
)

func (u *Usecases) SetUpLoadbalancers(request *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	logger := commonUtils.CreateLoggerWithProjectAndClusterName(request.ProjectName, commonUtils.GetClusterID(request.Desired.ClusterInfo))
	logger.Info().Msgf("Setting up the loadbalancers")

	currentLBClusters := make(map[string]*pb.LBcluster)
	for _, lbCluster := range request.CurrentLbs {
		currentLBClusters[lbCluster.ClusterInfo.Name] = lbCluster
	}

	lbClustersInfo := &utils.LBClustersInfo{
		FirstRun:              request.FirstRun,
		TargetK8sNodepool:     request.Desired.ClusterInfo.NodePools,
		TargetK8sNodepoolKey:  request.Desired.ClusterInfo.PrivateKey,
		PreviousAPIEndpointLB: request.PreviousAPIEndpoint,
		ClusterID:             fmt.Sprintf("%s-%s", request.Desired.ClusterInfo.Name, request.Desired.ClusterInfo.Hash),
	}
	for _, lbCluster := range request.DesiredLbs {
		lbClustersInfo.LbClusters = append(lbClustersInfo.LbClusters, &utils.LBClusterData{
			DesiredLbCluster: lbCluster,
			// if there is a value in the map it will return it, otherwise nil is returned.
			CurrentLbCluster: currentLBClusters[lbCluster.ClusterInfo.Name],
		})
	}

	if err := setUpLoadbalancers(request.Desired.ClusterInfo.Name, lbClustersInfo, logger); err != nil {
		logger.Err(err).Msgf("Error encountered while setting up the loadbalancers")
		return nil, fmt.Errorf("error encountered while setting up the loadbalancers for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("Loadbalancers were successfully set up")
	return &pb.SetUpLBResponse{Desired: request.Desired, CurrentLbs: request.CurrentLbs, DesiredLbs: request.DesiredLbs}, nil
}

// setUpLoadbalancers sets up the loadbalancers along with DNS and verifies their configuration
func setUpLoadbalancers(clusterName string, lbClustersInfo *utils.LBClustersInfo, logger zerolog.Logger) error {
	clusterBaseDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", clusterName, commonUtils.CreateHash(commonUtils.HashLength)))

	if err := utils.GenerateLBBaseFiles(clusterBaseDirectory, lbClustersInfo); err != nil {
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
			clusterDirectory := filepath.Join(clusterBaseDirectory, lbClusterId)
			if err := commonUtils.CreateDirectory(clusterDirectory); err != nil {
				return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
			}

			// Generate SSH key
			if err := commonUtils.CreateKeyFile(lbCluster.DesiredLbCluster.ClusterInfo.PrivateKey, clusterDirectory, fmt.Sprintf("key.%s", sshPrivateKeyFileExtension)); err != nil {
				return fmt.Errorf("failed to create key file for %s : %w", lbCluster.DesiredLbCluster.ClusterInfo.Name, err)
			}

			if err := setUpNodeExporter(lbCluster.DesiredLbCluster, clusterDirectory); err != nil {
				return err
			}

			if err := setUpNginx(lbCluster.DesiredLbCluster, lbClustersInfo.TargetK8sNodepool, clusterDirectory); err != nil {
				return err
			}

			logger.Info().Str(loggerPrefix, lbClusterId).Msg("Loadbalancer cluster successfully set up")
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("error while setting up the loadbalancers for cluster %s : %w", clusterName, err)
	}

	var desiredApiServerTypeLBCluster *utils.LBClusterData
	for _, lbClusterInfo := range lbClustersInfo.LbClusters {
		if commonUtils.HasAPIServerRole(lbClusterInfo.DesiredLbCluster.Roles) {
			desiredApiServerTypeLBCluster = lbClusterInfo
		}
	}

	// If we didn't find any Api server type LB cluster in the desired state,
	// it's possible that we've changed the role from an API server to some other role.
	// This won't be caught by the above check.
	// So we have to do an additional check for the desiredApiServerTypeLBCluster using the current state.
	if desiredApiServerTypeLBCluster == nil {
		desiredApiServerTypeLBCluster = utils.FindCurrentAPIServerTypeLBCluster(lbClustersInfo.LbClusters)
	}

	if err := utils.HandleAPIEndpointChange(desiredApiServerTypeLBCluster, lbClustersInfo, clusterBaseDirectory); err != nil {
		return fmt.Errorf("failed to find a candidate for the Api Server: %w", err)
	}

	return os.RemoveAll(clusterBaseDirectory)
}

// setUpNodeExporter sets up node-exporter on each node of the LB cluster.
// Returns error if not successful, nil otherwise.
func setUpNodeExporter(lbCluster *pb.LBcluster, clusterDirectory string) error {
	var playbookParameters = utils.LBPlaybookParameters{Loadbalancer: lbCluster.ClusterInfo.Name}

	// Generate node-exporter Ansible playbook from template
	template, err := templateUtils.LoadTemplate(templates.NodeExporterPlaybookTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s template for node_exporter playbook : %w", lbCluster.ClusterInfo.Name, err)
	}
	if err := (templateUtils.Templates{Directory: clusterDirectory}).Generate(template, nodeExporterPlaybookFileName, playbookParameters); err != nil {
		return fmt.Errorf("error while generating %s for %s : %w", nodeExporterPlaybookFileName, lbCluster.ClusterInfo.Name, err)
	}

	// Run the Ansible playbook
	ansible := utils.Ansible{
		Directory: clusterDirectory,
		Playbook:  nodeExporterPlaybookFileName,
		Inventory: filepath.Join("..", utils.InventoryFileName),
	}
	if err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

// setUpNginx sets up the nginx loadbalancer based on the input manifest specification.
// Return error if not successful, nil otherwise
func setUpNginx(lbCluster *pb.LBcluster, targetK8sNodepool []*pb.NodePool, clusterDirectory string) error {
	targetControlNodes, targetComputeNodes := splitNodesByType(targetK8sNodepool)

	// construct []LBClusterRolesInfo for the given LB cluster
	var lbClusterRolesInfo []utils.LBClusterRolesInfo
	for _, role := range lbCluster.Roles {
		target := assignTarget(targetControlNodes, targetComputeNodes, role.Target)

		if target == nil {
			return fmt.Errorf("target %v did not specify any nodes", role.Target)
		}
		lbClusterRolesInfo = append(lbClusterRolesInfo, utils.LBClusterRolesInfo{Role: role, TargetNodes: target})
	}

	// Generate the nginx config file
	nginxConfTemplate, err := templateUtils.LoadTemplate(templates.NginxConfigTemplate)
	if err != nil {
		return fmt.Errorf("error while loading nginx config template : %w", err)
	}
	err = (templateUtils.Templates{Directory: clusterDirectory}).
		Generate(nginxConfTemplate, "lb.conf", utils.NginxConfigTemplateParameters{Roles: lbClusterRolesInfo})
	if err != nil {
		return fmt.Errorf("error while generating lb.conf for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	ansible := utils.Ansible{
		Playbook:  nginxPlaybookName,
		Inventory: filepath.Join("..", utils.InventoryFileName),
		Directory: clusterDirectory,
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
