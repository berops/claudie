package usecases

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

const (
	// nodeExporterPlaybookFileName defines name for node exporter playbook.
	nodeExporterPlaybookFileName = "node-exporter.yml"
	// nginxPlaybookName defines name for nginx playbook.
	nginxPlaybookName = "nginx.yml"
	// nginxConfigName defines name for nginx config.
	nginxConfigName = "lb.conf"
)

func (u *Usecases) SetUpLoadbalancers(request *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	logger := loggerutils.WithProjectAndCluster(request.ProjectName, request.Desired.ClusterInfo.Id())
	logger.Info().Msgf("Setting up the loadbalancers")

	if err := setUpLoadbalancers(request, logger, u.SpawnProcessLimit); err != nil {
		logger.Err(err).Msgf("Error encountered while setting up the loadbalancers")
		return nil, fmt.Errorf("error encountered while setting up the loadbalancers for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("Loadbalancers were successfully set up")
	return &pb.SetUpLBResponse{Desired: request.Desired, DesiredLbs: request.DesiredLbs}, nil
}

// setUpLoadbalancers sets up the loadbalancers along with DNS and verifies their configuration
func setUpLoadbalancers(request *pb.SetUpLBRequest, logger zerolog.Logger, processLimit *semaphore.Weighted) error {
	clusterName := request.Desired.ClusterInfo.Id()
	clusterBaseDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", clusterName, hash.Create(hash.Length)))

	info := &utils.LBClustersInfo{
		Lbs:               request.DesiredLbs,
		TargetK8sNodepool: request.Desired.ClusterInfo.NodePools,
		ClusterID:         request.Desired.ClusterInfo.Id(),
	}

	if err := utils.GenerateLBBaseFiles(clusterBaseDirectory, info); err != nil {
		return fmt.Errorf("error encountered while generating base files for %s : %w", clusterName, err)
	}

	err := concurrent.Exec(info.Lbs, func(_ int, lbCluster *spec.LBcluster) error {
		var (
			loggerPrefix = "LB-cluster"
			lbClusterId  = lbCluster.ClusterInfo.Id()
		)

		logger.Info().Str(loggerPrefix, lbClusterId).Msg("Setting up the loadbalancer cluster")

		// Create the directory where files will be generated
		clusterDirectory := filepath.Join(clusterBaseDirectory, lbClusterId)
		if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
			return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
		}

		if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(lbCluster.ClusterInfo.NodePools), clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
		}

		if err := nodepools.StaticGenerateKeys(nodepools.Static(lbCluster.ClusterInfo.NodePools), clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
		}

		if err := setUpNodeExporter(lbCluster, clusterDirectory, processLimit); err != nil {
			return err
		}

		if err := setUpNginx(lbCluster, info.TargetK8sNodepool, clusterDirectory, processLimit); err != nil {
			return err
		}

		logger.Info().Str(loggerPrefix, lbClusterId).Msg("Loadbalancer cluster successfully set up")
		return nil
	})
	if err != nil {
		return fmt.Errorf("error while setting up the loadbalancers for cluster %s : %w", clusterName, err)
	}

	if err := handleMoveApiEndpoint(logger, request, clusterBaseDirectory, processLimit); err != nil {
		return err
	}

	return os.RemoveAll(clusterBaseDirectory)
}

// setUpNodeExporter sets up node-exporter on each node of the LB cluster.
// Returns error if not successful, nil otherwise.
func setUpNodeExporter(lbCluster *spec.LBcluster, clusterDirectory string, processLimit *semaphore.Weighted) error {
	var playbookParameters = utils.LBPlaybookParameters{Loadbalancer: lbCluster.ClusterInfo.Name}

	// Generate node-exporter Ansible playbook from template
	template, err := templateUtils.LoadTemplate(templates.NodeExporterPlaybookTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s template for node_exporter playbook : %w", lbCluster.ClusterInfo.Name, err)
	}

	tpl := templateUtils.Templates{Directory: clusterDirectory}
	if err := tpl.Generate(template, nodeExporterPlaybookFileName, playbookParameters); err != nil {
		return fmt.Errorf("error while generating %s for %s : %w", nodeExporterPlaybookFileName, lbCluster.ClusterInfo.Name, err)
	}

	// Run the Ansible playbook
	ansible := utils.Ansible{
		Directory:         clusterDirectory,
		Playbook:          nodeExporterPlaybookFileName,
		Inventory:         filepath.Join("..", utils.InventoryFileName),
		SpawnProcessLimit: processLimit,
	}

	if err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

// setUpNginx sets up the nginx loadbalancer based on the input manifest specification.
// Return error if not successful, nil otherwise
func setUpNginx(lbCluster *spec.LBcluster, targetK8sNodepool []*spec.NodePool, clusterDirectory string, processLimit *semaphore.Weighted) error {
	lbClusterRolesInfo := targetPools(lbCluster, targetK8sNodepool)
	// Generate the nginx config file
	nginxConfTemplate, err := templateUtils.LoadTemplate(templates.NginxConfigTemplate)
	tpl := templateUtils.Templates{Directory: clusterDirectory}
	if err != nil {
		return fmt.Errorf("error while loading nginx config template : %w", err)
	}
	nginxPlaybookTemplate, err := templateUtils.LoadTemplate(templates.NginxPlaybookTemplate)
	if err != nil {
		return fmt.Errorf("error while loading nginx playbook template : %w", err)
	}

	if err := tpl.Generate(nginxConfTemplate, nginxConfigName, utils.NginxConfigTemplateParameters{Roles: lbClusterRolesInfo}); err != nil {
		return fmt.Errorf("error while generating %s for %s : %w", nginxConfigName, lbCluster.ClusterInfo.Name, err)
	}

	if err := tpl.Generate(nginxPlaybookTemplate, nginxPlaybookName, utils.LBPlaybookParameters{Loadbalancer: lbCluster.ClusterInfo.Name}); err != nil {
		return fmt.Errorf("error while generating %s for %s : %w", nginxPlaybookName, lbCluster.ClusterInfo.Name, err)
	}

	ansible := utils.Ansible{
		Playbook:          nginxPlaybookName,
		Inventory:         filepath.Join("..", utils.InventoryFileName),
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash))
	if err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

func targetPools(lbCluster *spec.LBcluster, targetK8sNodepool []*spec.NodePool) []utils.LBClusterRolesInfo {
	var lbClusterRolesInfo []utils.LBClusterRolesInfo
	for _, role := range lbCluster.Roles {
		lbClusterRolesInfo = append(lbClusterRolesInfo, utils.LBClusterRolesInfo{
			Role:        role,
			TargetNodes: targetNodes(role.TargetPools, targetK8sNodepool),
		})
	}

	return lbClusterRolesInfo
}

func targetNodes(targetPools []string, targetk8sPools []*spec.NodePool) (nodes []*spec.Node) {
	var pools []*spec.NodePool

	for _, target := range targetPools {
		for _, np := range targetk8sPools {
			if np.GetDynamicNodePool() != nil {
				if name, _ := nodepools.MatchNameAndHashWithTemplate(target, np.Name); name != "" {
					pools = append(pools, np)
				}
			} else if np.GetStaticNodePool() != nil {
				if target == np.Name {
					pools = append(pools, np)
				}
			}
		}
	}

	for _, np := range pools {
		nodes = append(nodes, np.Nodes...)
	}

	return
}

func handleMoveApiEndpoint(logger zerolog.Logger, request *pb.SetUpLBRequest, outputDirectory string, processLimit *semaphore.Weighted) error {
	cID, dID, state := clusters.DetermineLBApiEndpointChange(request.CurrentLbs, request.DesiredLbs)
	// Endpoint renamed has to be done at this stage, as the endpoint
	// was updated in the terraformer stage and needs to be subsequently
	// updated in ansibler.
	if state != spec.ApiEndpointChangeState_EndpointRenamed {
		return nil
	}

	lbc := clusters.IndexLoadbalancerById(cID, request.CurrentLbs)
	lbd := clusters.IndexLoadbalancerById(dID, request.DesiredLbs)

	if lbc < 0 {
		return fmt.Errorf("failed to find requested loadbalancer %s from which to move the api endpoint from", cID)
	}

	if lbd < 0 {
		return fmt.Errorf("failed to find requested loadbalancer %s to which to move the api endpoint", dID)
	}

	oldEndpoint := request.CurrentLbs[lbc].Dns.Endpoint
	newEndpoint := request.DesiredLbs[lbd].Dns.Endpoint

	cdid := clusters.IndexLoadbalancerById(cID, request.DesiredLbs)

	request.DesiredLbs[cdid].UsedApiEndpoint = false
	request.DesiredLbs[lbd].UsedApiEndpoint = true

	logger.Debug().Msgf("Changing the API endpoint from %s to %s", oldEndpoint, newEndpoint)

	request.ProxyEnvs.NoProxyList = strings.ReplaceAll(request.ProxyEnvs.NoProxyList, oldEndpoint, newEndpoint)

	err := utils.ChangeAPIEndpoint(
		request.Desired.ClusterInfo.Id(),
		oldEndpoint,
		newEndpoint,
		outputDirectory,
		request.ProxyEnvs,
		processLimit,
	)
	if err != nil {
		return fmt.Errorf("failed to change API endpoint from %s to %s: %w", oldEndpoint, newEndpoint, err)
	}

	return nil
}
