package usecases

import (
	"fmt"
	"os"
	"path/filepath"

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
	nodeExporterPlaybookName = "node-exporter.yml"

	// uninstallNginx playbook.
	uninstallNginxPlaybookName = "uninstall-nginx.yml"

	// envoyPlaybookName to which the template will be generated to
	// for docker and envoy proxy setup.
	envoyPlaybookName = "envoy.yml"

	// envoyDockerCompose is the generated compose file for all of the roles
	// for a single load-balancer that will be used to orchestrate the
	// listeners on the deployed load-balancer nodes.
	envoyDockerCompose = "envoy-docker-compose.yml"

	// envoyConfig is the generated config for a config for a single role that
	// dynamically updates cds and lds.
	envoyConfig = "envoy_temp.yml"

	// envoyCDS is the generated dynamic clusters config for a single role.
	envoyCDS = "cds_temp.yml"

	// envoyLDS is the generated dynamic listeners config for a single role.
	envoyLDS = "lds_temp.yml"
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

	defer func() {
		if err := os.RemoveAll(clusterBaseDirectory); err != nil {
			logger.Err(err).Msg("failed to clear up loadbalancer directory")
		}
	}()

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

		// TODO: remove.
		// For older claudie version which deployed nginx as the loadbalancer uninstall the service.
		// this is a one time update that will introduce a small downtime of the services while nginx is being replaced.
		// subsequent execution of the playbook will error out, but the error will be ignored.
		if err := uninstallNginx(lbCluster, clusterDirectory, processLimit); err != nil {
			return err
		}

		if err := setupEnvoyProxyViaDocker(lbCluster, info.TargetK8sNodepool, clusterDirectory, processLimit); err != nil {
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

	return nil
}

// setUpNodeExporter sets up node-exporter on each node of the LB cluster.
// Returns error if not successful, nil otherwise.
func setUpNodeExporter(lbCluster *spec.LBcluster, clusterDirectory string, processLimit *semaphore.Weighted) error {
	playbookParameters := utils.NodeExporterTamplateParams{
		LoadBalancer: lbCluster.ClusterInfo.Name,
	}

	template, err := templateUtils.LoadTemplate(templates.NodeExporterPlaybookTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s template for node_exporter playbook : %w", lbCluster.ClusterInfo.Name, err)
	}

	tpl := templateUtils.Templates{Directory: clusterDirectory}
	if err := tpl.Generate(template, nodeExporterPlaybookName, playbookParameters); err != nil {
		return fmt.Errorf("error while generating %s for %s : %w", nodeExporterPlaybookName, lbCluster.ClusterInfo.Name, err)
	}

	ansible := utils.Ansible{
		Directory:         clusterDirectory,
		Playbook:          nodeExporterPlaybookName,
		Inventory:         filepath.Join("..", utils.InventoryFileName),
		SpawnProcessLimit: processLimit,
	}

	if err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lbCluster.ClusterInfo.Name, err)
	}
	return nil
}

func uninstallNginx(
	lbCluster *spec.LBcluster,
	clusterDirectory string,
	processLimit *semaphore.Weighted,
) error {
	tpl := templateUtils.Templates{Directory: clusterDirectory}
	uninstall, err := templateUtils.LoadTemplate(templates.UninstallNginx)
	if err != nil {
		return fmt.Errorf("error while loading nginx uninstall file for %s: %w", lbCluster.ClusterInfo.Id(), err)
	}

	err = tpl.Generate(uninstall, uninstallNginxPlaybookName, utils.UninstallNginxParams{
		LoadBalancer: lbCluster.ClusterInfo.Name,
	})

	if err != nil {
		return fmt.Errorf("error while generating uninstall nginx playbook for %s: %w", lbCluster.ClusterInfo.Id(), err)
	}

	ansible := utils.Ansible{
		RetryCount:        2,
		Playbook:          uninstallNginxPlaybookName,
		Inventory:         filepath.Join("..", utils.InventoryFileName),
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s", lbCluster.ClusterInfo.Id()))
	if err != nil {
		return fmt.Errorf("error while running ansible for %s: %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

// Installs or updates Docker and Docker Compose for setting up the load balancing functionality.
// Each defined "role" of the load balancer cluster is deployed as a separate Docker container.
// These containers are orchestrated using a Docker Compose file specific to the load balancer.
//
// The load balancer uses a dynamic configuration setup that includes dynamic clusters and listeners.
// These configurations can be updated at runtime without interrupting existing connections.
// This allows connections established under the old configuration to remain active, while new
// connections can use the updated configuration, enabling both to coexist seamlessly.
//
// Based on https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-dynamic-filesystem
func setupEnvoyProxyViaDocker(
	lbCluster *spec.LBcluster,
	targetK8sNodePool []*spec.NodePool,
	clusterDirectory string,
	processLimit *semaphore.Weighted,
) error {
	targets := targetPools(lbCluster, targetK8sNodePool)
	// generate per-role dynamic configs
	for _, tg := range targets {
		dir := filepath.Join(clusterDirectory, tg.Role.Name)
		if err := fileutils.CreateDirectory(dir); err != nil {
			return fmt.Errorf("failed to create directory for envoy config for role %s for cluster %s: %w", tg.Role.Name, lbCluster.ClusterInfo.Id(), err)
		}
		tpl := templateUtils.Templates{Directory: dir}

		dynClusters, err := templateUtils.LoadTemplate(templates.EnvoyDynamicClusters)
		if err != nil {
			return fmt.Errorf("error while loading dynamic clusters config template for %s: %w", lbCluster.ClusterInfo.Id(), err)
		}

		dynListeners, err := templateUtils.LoadTemplate(templates.EnvoyDynamicListeners)
		if err != nil {
			return fmt.Errorf("error while loading dynamic listeners config template for %s: %w", lbCluster.ClusterInfo.Id(), err)
		}

		envoy, err := templateUtils.LoadTemplate(templates.EnvoyConfig)
		if err != nil {
			return fmt.Errorf("error while loading envoy config template for %s: %w", lbCluster.ClusterInfo.Id(), err)
		}

		err = tpl.Generate(dynClusters, envoyCDS, utils.LBClusterRolesInfo{
			Role:        tg.Role,
			TargetNodes: tg.TargetNodes,
		})
		if err != nil {
			return fmt.Errorf("error while generating envoy dynamic clusters config for %s: %w", lbCluster.ClusterInfo.Id(), err)
		}

		err = tpl.Generate(dynListeners, envoyLDS, utils.LBClusterRolesInfo{
			Role:        tg.Role,
			TargetNodes: tg.TargetNodes,
		})
		if err != nil {
			return fmt.Errorf("error while generating envoy dynamic listeners config for %s: %w", lbCluster.ClusterInfo.Id(), err)
		}

		err = tpl.Generate(envoy, envoyConfig, utils.EnvoyTemplateParams{
			LoadBalancer: lbCluster.ClusterInfo.Name,
			Role:         tg.Role.Name,
		})
		if err != nil {
			return fmt.Errorf("error while generatingf envoy config for %s: %w", lbCluster.ClusterInfo.Id(), err)
		}
	}

	tpl := templateUtils.Templates{Directory: clusterDirectory}

	// generate compose file.
	compose, err := templateUtils.LoadTemplate(templates.EnvoyDockerCompose)
	if err != nil {
		return fmt.Errorf("error while loading envoy compose file for %s: %w", lbCluster.ClusterInfo.Id(), err)
	}

	err = tpl.Generate(compose, envoyDockerCompose, utils.EnvoyConfigTemplateParams{
		LoadBalancer: lbCluster.ClusterInfo.Name,
		Roles:        targets,
	})
	if err != nil {
		return fmt.Errorf("error while generating envoy docker compose file for %s: %w", lbCluster.ClusterInfo.Id(), err)
	}

	// install docker/docker-compose on the nodes, upload the config and deploy envoy.
	envoyPlayBook, err := templateUtils.LoadTemplate(templates.EnvoyTemplate)
	if err != nil {
		return fmt.Errorf("error while loading docker template: %w", err)
	}

	err = tpl.Generate(envoyPlayBook, envoyPlaybookName, utils.EnvoyConfigTemplateParams{
		LoadBalancer: lbCluster.ClusterInfo.Name,
		Roles:        targets,
	})
	if err != nil {
		return fmt.Errorf("error while generating  %s for %s: %w", envoyPlaybookName, lbCluster.ClusterInfo.Name, err)
	}

	ansible := utils.Ansible{
		RetryCount:        2,
		Playbook:          envoyPlaybookName,
		Inventory:         filepath.Join("..", utils.InventoryFileName),
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash))
	if err != nil {
		return fmt.Errorf("error while running ansible for %s: %w", lbCluster.ClusterInfo.Name, err)
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

	err := utils.ChangeAPIEndpoint(
		request.Desired.ClusterInfo.Id(),
		oldEndpoint,
		newEndpoint,
		outputDirectory,
		processLimit,
	)
	if err != nil {
		return fmt.Errorf("failed to change API endpoint from %s to %s: %w", oldEndpoint, newEndpoint, err)
	}

	return nil
}
