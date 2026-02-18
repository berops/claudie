package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	utils "github.com/berops/claudie/services/ansibler/internal/worker/service/internal"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

const (
	// nodeExporterPlaybookFileName defines name for node exporter playbook.
	nodeExporterPlaybookName = "node-exporter.yml"

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

type (
	LoadBalancersData struct {
		Clusters             []*spec.LBcluster
		TargetedK8sNodePools []*spec.NodePool
		K8sClusterId         string
	}

	LBcluster struct {
		Name        string
		Hash        string
		LBnodepools NodePools
	}

	UninstallNginxParams struct {
		LoadBalancer string
	}

	NodeExporterTemplateParams struct {
		LoadBalancer     string
		NodeExporterPort int
	}

	RolesTemplateParams struct {
		Role        *spec.Role
		TargetNodes []*spec.Node
	}

	EnvoyConfigTemplateParams struct {
		LoadBalancer string
		Roles        []RolesTemplateParams
	}

	EnvoyTemplateParams struct {
		LoadBalancer   string
		Role           string
		EnvoyAdminPort int32
	}
)

func ReconcileLoadBalancers(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Reconciling LoadBalancers")

	var (
		k8s *spec.K8Scluster
		lbs []*spec.LBcluster

		// unreachable infrastructure, if any, that will be skipped
		// during the installation of the VPN.
		unreachable *spec.Unreachable
	)

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = do.Create.K8S
		lbs = do.Create.LoadBalancers
	case *spec.Task_Update:
		k8s = do.Update.State.K8S

		// Will default to only reconciling a single loadbalancer, if possible.
		// But will reconcile all of the nodepools of that loadbalancer.
		//
		// This could be further, changed to only consider changes within
		// a nodepool of the loadbalancer and ignore others unchanged, for
		// now leave this as is.
		if lb := DefaultToSingleLoadBalancerIfPossible(do); lb != nil {
			lbs = []*spec.LBcluster{lb}
		} else {
			lbs = do.Update.State.LoadBalancers
		}

		unreachable = UnreachableInfrastructure(do)

		// In case target pools are required to be replaced, replace them first
		// before executing the ansible playbook which will result in updating
		// the generated envoy configs for the new nodepools.
		tg, ok := do.Update.Delta.(*spec.Update_AnsReplaceTargetPools)
		if ok {
			idx := clusters.IndexLoadbalancerById(tg.AnsReplaceTargetPools.Handle, lbs)
			if idx >= 0 {
				for _, role := range lbs[idx].Roles {
					if n, ok := tg.AnsReplaceTargetPools.Roles[role.Name]; ok {
						role.TargetPools = n.Pools
					}
				}
			}
		}

		// same as with replace target pools.
		st, ok := do.Update.Delta.(*spec.Update_AnsReplaceRoleInternalSettings)
		if ok {
			idx := clusters.IndexLoadbalancerById(st.AnsReplaceRoleInternalSettings.Handle, lbs)
			if idx >= 0 {
				for _, role := range lbs[idx].Roles {
					if role.Name == st.AnsReplaceRoleInternalSettings.Role {
						role.TargetPort = st.AnsReplaceRoleInternalSettings.TargetPort
						role.Settings = st.AnsReplaceRoleInternalSettings.Settings
					}
				}
			}
		}
	default:
		logger.
			Warn().
			Msgf(
				"received task with action %T while wanting to reconcile loadbalancers, assuming the task was misscheduled, ignoring",
				tracker.Task.GetDo(),
			)
		return
	}

	clusterId := k8s.ClusterInfo.Id()
	data := LoadBalancersData{
		Clusters:             lbs,
		TargetedK8sNodePools: k8s.ClusterInfo.NodePools,
		K8sClusterId:         clusterId,
	}

	if err := setUpLoadbalancers(logger, data, unreachable, processLimit); err != nil {
		logger.Err(err).Msg("Failed to setup loadbalancers")
		tracker.Diagnostics.Push(err)
		return
	}

	// If replacement was done as part of a scheduled update, report back the changes.
	switch {
	case
		tracker.Task.GetUpdate().GetAnsReplaceTargetPools() != nil,
		tracker.Task.GetUpdate().GetAnsReplaceRoleInternalSettings() != nil:

		update := tracker.Result.Update()
		update.Loadbalancers(lbs...)
		update.Commit()
	}

	logger.Info().Msg("Successfully reconciled LoadBalancers")
}

// setUpLoadbalancers sets up the loadbalancers along with DNS and verifies their configuration
func setUpLoadbalancers(
	logger zerolog.Logger,
	ci LoadBalancersData,
	unreachable *spec.Unreachable,
	processLimit *semaphore.Weighted,
) error {
	clusterBaseDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s-lbs", ci.K8sClusterId, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterBaseDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterBaseDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterBaseDirectory); err != nil {
			logger.Err(err).Msg("failed to clear up loadbalancer directory")
		}
	}()

	return concurrent.Exec(ci.Clusters, func(_ int, lbCluster *spec.LBcluster) error {
		var (
			loggerPrefix = "LB-cluster"
			lbClusterId  = lbCluster.ClusterInfo.Id()
			logger       = logger.With().Str(loggerPrefix, lbClusterId).Logger()
			lbnps        = lbCluster.ClusterInfo.NodePools
		)

		logger.Info().Msg("Setting up the loadbalancer cluster")

		clusterDirectory := filepath.Join(clusterBaseDirectory, lbClusterId)
		if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
			return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
		}

		if unreachable != nil {
			lbnps = DefaultNodePoolsToReachableInfrastructureOnly(
				lbnps,
				// The playbook is only executed on loadbalancers.
				unreachable.Loadbalancers[lbClusterId],
			)
		}

		dynamic := nodepools.Dynamic(lbnps)
		static := nodepools.Static(lbnps)
		data := LBcluster{
			Name: lbCluster.ClusterInfo.Name,
			Hash: lbCluster.ClusterInfo.Hash,
			LBnodepools: NodePools{
				Dynamic: dynamic,
				Static:  static,
			},
		}

		err := utils.GenerateInventoryFile(templates.LoadbalancerInventoryTemplate, clusterDirectory, data)
		if err != nil {
			return fmt.Errorf("error while generating inventory file for %s : %w", clusterDirectory, err)
		}

		if err := nodepools.DynamicGenerateKeys(dynamic, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
		}

		if err := nodepools.StaticGenerateKeys(static, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
		}

		if err := setUpNodeExporter(lbCluster, clusterDirectory, processLimit); err != nil {
			return err
		}

		if err := setupEnvoyProxyViaDocker(lbCluster, ci.TargetedK8sNodePools, clusterDirectory, processLimit); err != nil {
			return err
		}

		logger.Info().Msg("Loadbalancer cluster successfully set up")
		return nil
	})
}

// setUpNodeExporter sets up node-exporter on each node of the LB cluster.
func setUpNodeExporter(lbCluster *spec.LBcluster, clusterDirectory string, processLimit *semaphore.Weighted) error {
	playbookParameters := NodeExporterTemplateParams{
		LoadBalancer:     lbCluster.ClusterInfo.Name,
		NodeExporterPort: manifest.NodeExporterPort,
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
		Inventory:         utils.InventoryFileName,
		SpawnProcessLimit: processLimit,
	}

	if err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", lbCluster.ClusterInfo.Name, err)
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
	clusterId := lbCluster.ClusterInfo.Id()
	targets := roleTargetPools(lbCluster, targetK8sNodePool)

	// generate per-role dynamic configs
	for _, tg := range targets {
		dir := filepath.Join(clusterDirectory, tg.Role.Name)
		if err := fileutils.CreateDirectory(dir); err != nil {
			return fmt.Errorf("failed to create directory for envoy config for role %s for cluster %s: %w", tg.Role.Name, clusterId, err)
		}

		cds := templates.EnvoyDynamicClusters
		lds := templates.EnvoyDynamicListeners

		dynClusters, err := templateUtils.LoadTemplate(cds)
		if err != nil {
			return fmt.Errorf("error while loading dynamic clusters config template for %s: %w", clusterId, err)
		}

		dynListeners, err := templateUtils.LoadTemplate(lds)
		if err != nil {
			return fmt.Errorf("error while loading dynamic listeners config template for %s: %w", clusterId, err)
		}

		envoy, err := templateUtils.LoadTemplate(templates.EnvoyConfig)
		if err != nil {
			return fmt.Errorf("error while loading envoy config template for %s: %w", clusterId, err)
		}

		rolesData := RolesTemplateParams{
			Role:        tg.Role,
			TargetNodes: tg.TargetNodes,
		}

		envoyData := EnvoyTemplateParams{
			LoadBalancer:   lbCluster.ClusterInfo.Name,
			Role:           tg.Role.Name,
			EnvoyAdminPort: tg.Role.Settings.EnvoyAdminPort,
		}

		tpl := templateUtils.Templates{Directory: dir}

		if err := tpl.Generate(dynClusters, envoyCDS, rolesData); err != nil {
			return fmt.Errorf(
				"error while generating envoy dynamic clusters config for %s: %w",
				clusterId,
				err,
			)
		}

		if err := tpl.Generate(dynListeners, envoyLDS, rolesData); err != nil {
			return fmt.Errorf(
				"error while generating envoy dynamic listeners config for %s: %w",
				clusterId,
				err,
			)
		}

		if err := tpl.Generate(envoy, envoyConfig, envoyData); err != nil {
			return fmt.Errorf("error while generatingf envoy config for %s: %w", clusterId, err)
		}
	}

	tpl := templateUtils.Templates{Directory: clusterDirectory}

	// generate compose file.
	compose, err := templateUtils.LoadTemplate(templates.EnvoyDockerCompose)
	if err != nil {
		return fmt.Errorf("error while loading envoy compose file for %s: %w", clusterId, err)
	}

	// install docker/docker-compose on the nodes, upload the config and deploy envoy.
	envoyPlayBook, err := templateUtils.LoadTemplate(templates.EnvoyTemplate)
	if err != nil {
		return fmt.Errorf("error while loading docker template: %w", err)
	}

	envoyData := EnvoyConfigTemplateParams{
		LoadBalancer: lbCluster.ClusterInfo.Name,
		Roles:        targets,
	}

	if err := tpl.Generate(compose, envoyDockerCompose, envoyData); err != nil {
		return fmt.Errorf("error while generating envoy docker compose file for %s: %w", clusterId, err)
	}

	if err := tpl.Generate(envoyPlayBook, envoyPlaybookName, envoyData); err != nil {
		return fmt.Errorf("error while generating  %s for %s: %w", envoyPlaybookName, clusterId, err)
	}

	ansible := utils.Ansible{
		RetryCount:        2,
		Playbook:          envoyPlaybookName,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("LB - %s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash))
	if err != nil {
		return fmt.Errorf("error while running ansible for %s: %w", lbCluster.ClusterInfo.Name, err)
	}

	return nil
}

func roleTargetPools(lbCluster *spec.LBcluster, targetK8sNodepool []*spec.NodePool) (ri []RolesTemplateParams) {
	for _, role := range lbCluster.Roles {
		pools := targetK8sNodepool
		if role.RoleType == spec.RoleType_ApiServer {
			// If the target pools of the role are also re-used
			// in the worker nodepools for the api server only
			// consider control pools.
			pools = slices.Collect(nodepools.Control(pools))
		}

		ri = append(ri, RolesTemplateParams{
			Role:        role,
			TargetNodes: targetNodes(role.TargetPools, pools),
		})
	}

	return
}

func targetNodes(targetPools []string, targetk8sPools []*spec.NodePool) (nodes []*spec.Node) {
	var pools []*spec.NodePool

	for _, target := range targetPools {
		for _, np := range targetk8sPools {
			if np.GetDynamicNodePool() != nil {
				if nodepools.HasNodePoolTypeOf(target, np.Name) {
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
