package service

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
)

const (
	// CIDR used when creating the nodepools for desired state.
	baseSubnetCIDR       = "10.0.0.0/24"
	defaultOctetToChange = 2
)

func createDesiredState(pending *spec.ConfigV2, result *map[string]*spec.ClustersV2) error {
	if result == nil {
		return errors.New("empty pointer to map for clusters desired state")
	}

	// 1. If the infrastructure was marked for deletion the InputManifest and the ManifestChecksum will be empty.
	markedForDeletion := pending.Manifest.Raw == "" &&
		pending.Manifest.Checksum == nil &&
		pending.Manifest.State == spec.ManifestV2_Pending

	if markedForDeletion {
		// The result map will be empty, indicating there is no desired state.
		clear(*result)
		return nil
	}

	// 2. create the desired state from the input manifest.
	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(pending.Manifest.Raw), &m); err != nil {
		return fmt.Errorf("error unmarshalling manifest for config %q: %w", pending.Name, err)
	}

	var desiredState map[string]*spec.ClustersV2
	if err := createK8sClustersFromManifest(&m, &desiredState); err != nil {
		return fmt.Errorf("failed to parse k8s clusters from manifest %q: %w", m.Name, err)
	}
	if err := createLBClustersFromManifest(&m, &desiredState); err != nil {
		return fmt.Errorf("failed to parse lb clusters from manifest: %q: %w", m.Name, err)
	}

	// TODO: double check and validate with tests the slightly changed build of desired state.
	// TODO: then use the transfering of the state when propagating the Task Result.
	// 3.
	// In the next steps It might be the case either the Current State or Desired state is nil
	// thus these cases needs to be handled gracefully. Parts of the desired state are populated
	// based on the values in the current state, if any exists.
	for cluster, desired := range desiredState {
		// It is guaranteed by validation, that within a single InputManifest
		// no two clusters (including LB) can share the same name.
		current := pending.Clusters[cluster].GetCurrent()

		// Generate hashes for multiple references of the same dynamic nodepools,
		// while also considering hashes previously assigned in the current state.
		//
		// This is so that the same reference to a dynamic nodepool is treated as
		// a unique nodepool with its own state to be maintained. The validation
		// layers guarantee that a dynamic nodepool can be referenced at most once
		// within a block, i.e kubernetes:, loadbalancers:, If that invariant is
		// somehow broken, the function panics.
		deduplicateDynamicNodepoolNames(&m, current, desired)

		// Deduplicate, static node names. Contrary to how dynamic node works, static
		// nodes are identified via the Public endpoint. The name that is assigned to
		// static nodes is not used to identify them, atleast not in this stage of the
		// build pipeline.
		//
		// For identifying the if the same static node is reused the public Endpoint is used.
		// After the [deduplicateStaticNodeNames] newly added static nodes will have unique
		// names
		deduplicateStaticNodeNames(current, desired)

		if current.GetK8S() != nil {
			log.Debug().Str("cluster", cluster).Msgf("reusing existing state")

			// Transfers any state from the [current] into [desired]. The state
			// that is transferred is considered to be "Immutable" by claudie,
			// thus once it is assigned it must not changed. This "Immutable"
			// state is used here to transfer it, possibly overwriting any
			// existing state in [desired].
			transferPreviouslyAcquiredState(current, desired)
		}

		// Must follow after [transferPreviouslyAcquiredState]
		// Generate the CIDR for individual nodepools at this step, as
		// we need information about the current state so that we do not
		// generate the same cidr for the same Provider/Region pair multiple
		// times, avoiding conflicts.
		if err := GenerateMissingCIDR(current, desired); err != nil {
			return fmt.Errorf("failed to generate CIDRs for nodepools: %w", err)
		}
	}

	// uses only current state, so we can pass [pending] without the [desiredState]
	// yet to be fully populated.
	backwardsCompatibility(pending)

	// 4. After step 3. there can still be missing state that needs to be populated.
	for _, desired := range desiredState {
		if err := PopulateSSHKeys(desired); err != nil {
			return err
		}

		// Populate DNS hostname if it was not transfered from the existing state.
		PopulateDnsHostName(desired)

		// After transferring the existing state we fill the remaining
		// missing dynamic nodes, as the [spec.DynamicNodePool.Count]
		// could have changed.
		PopulateDynamicNodesForClusters(desired)

		// validation of the Manifest assures that the number of
		// roles is limited and that we will always be able to
		// generate the required number of ports for the envoy
		// admin interface.
		PopulateEnvoyAdminPorts(desired)

		// To each loadbalancer cluster we add a default healthcheck role
		// that can then be used for the HA loadbalancing.
		PopulateDefaultHealthcheckRole(desired)
	}

	*result = desiredState
	return nil
}

// createK8sClustersFromManifest parses manifest and creates desired state for Kubernetes clusters.
// The desired state of the clusters is filled into the passed in `into` map. It is then necessary
// to compare the current state to the filled out desired state of the cluster to determine what changed.
func createK8sClustersFromManifest(from *manifest.Manifest, into *map[string]*spec.ClustersV2) error {
	if into == nil {
		return errors.New("empty pointer to map for clusters desired state")
	}
	if *into == nil {
		*into = make(map[string]*spec.ClustersV2)
	}
	clear(*into)

	// 1. traverse all clusters in the manifest
	//    catching newly created or existing (updated).
	for _, cluster := range from.Kubernetes.Clusters {
		useInstallationProxy := &spec.InstallationProxyV2{
			Mode: "default",
		}

		if cluster.InstallationProxy != nil {
			useInstallationProxy.Mode = cluster.InstallationProxy.Mode
			useInstallationProxy.Endpoint = cluster.InstallationProxy.Endpoint

			useInstallationProxy.NoProxy = strings.TrimSpace(cluster.InstallationProxy.NoProxy)
			useInstallationProxy.NoProxy = strings.Trim(useInstallationProxy.NoProxy, ",")
		}

		newCluster := &spec.K8SclusterV2{
			ClusterInfo: &spec.ClusterInfoV2{
				Name: strings.ToLower(cluster.Name),
				Hash: hash.Create(hash.Length),
			},
			Kubernetes:        cluster.Version,
			Network:           cluster.Network,
			InstallationProxy: useInstallationProxy,
		}

		controlNodePools, err := from.CreateNodepools(cluster.Pools.Control, true)
		if err != nil {
			return fmt.Errorf("error while creating control nodepool for %s : %w", cluster.Name, err)
		}

		computeNodePools, err := from.CreateNodepools(cluster.Pools.Compute, false)
		if err != nil {
			return fmt.Errorf("error while creating compute nodepool for %s : %w", cluster.Name, err)
		}

		newCluster.ClusterInfo.NodePools = append(controlNodePools, computeNodePools...)

		// NOTE: the CIDR and SSH keys are not populated at this point in the pipeline. Here
		// only the parsed skeleton of the passed in [manifest.Manifest] is created.

		(*into)[newCluster.ClusterInfo.Name] = &spec.ClustersV2{K8S: newCluster}
	}

	return nil
}

// createLBClustersFromManifest reads the manifest and creates load balancer clusters based on it.
// It continues to fill the map from the [createDesiredState] function with the matching loadbalancers.
func createLBClustersFromManifest(from *manifest.Manifest, into *map[string]*spec.ClustersV2) error {
	if into == nil {
		return errors.New("empty pointer to map for clusters desired state")
	}

	// 1. Collect all Lbs in the desired state for given K8s clusters.
	lbs := make(map[string]*spec.LoadBalancersV2)
	for _, lbCluster := range from.LoadBalancer.Clusters {
		dns, err := getDNS(lbCluster.DNS, from)
		if err != nil {
			return fmt.Errorf("error while building desired state for LB %s : %w", lbCluster.Name, err)
		}

		// NOTE: do not populate roles.Settings.EnvoyAdminPort at this stage.
		attachedRoles := getRolesAttachedToLBCluster(from.LoadBalancer.Roles, lbCluster.Roles)

		newLbCluster := &spec.LBclusterV2{
			ClusterInfo: &spec.ClusterInfoV2{
				Name: lbCluster.Name,
				Hash: hash.Create(hash.Length),
			},
			Roles:       attachedRoles,
			Dns:         dns,
			TargetedK8S: lbCluster.TargetedK8s,
		}

		nodes, err := from.CreateNodepools(lbCluster.Pools, false)
		if err != nil {
			return fmt.Errorf("error while creating nodepools for %s : %w", lbCluster.Name, err)
		}
		newLbCluster.ClusterInfo.NodePools = nodes

		// NOTE:
		// the CIDR,SSH keys and DNS hostname (if not set) are not populated at this point
		// in the pipeline. Here only the parsed skeleton of the passed in [manifest.Manifest]
		// is created.

		if _, ok := lbs[newLbCluster.TargetedK8S]; !ok {
			lbs[newLbCluster.TargetedK8S] = new(spec.LoadBalancersV2)
		}
		lbs[newLbCluster.TargetedK8S].Clusters = append(lbs[newLbCluster.TargetedK8S].Clusters, newLbCluster)
	}

	// 2. Marshal and match with respective clusters.
	for k8sCluster := range *into {
		lbs, ok := lbs[k8sCluster]
		if !ok {
			continue
		}
		(*into)[k8sCluster].LoadBalancers = lbs
	}

	return nil
}

// getDNS parses the manifest for the DNS specification.
func getDNS(dns manifest.DNS, from *manifest.Manifest) (*spec.DNS, error) {
	if dns.DNSZone == "" {
		return nil, fmt.Errorf("DNS zone not provided in manifest %s", from.Name)
	}

	provider, err := from.GetProvider(dns.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %s was not found in manifest %s: %w", dns.Provider, from.Name, err)
	}

	var alternativeNames []*spec.AlternativeName
	for _, n := range dns.AlternativeNames {
		alternativeNames = append(alternativeNames, &spec.AlternativeName{
			Hostname: n,
		})
	}

	return &spec.DNS{
		DnsZone:          dns.DNSZone,
		Provider:         provider,
		Hostname:         dns.Hostname,
		AlternativeNames: alternativeNames,
	}, nil
}

// getRolesAttachedToLBCluster will read roles attached to the LB cluster from the unmarshalled manifest and return them.
// Returns slice of *[]pb.Roles if successful, error if Target from manifest state not found
func getRolesAttachedToLBCluster(roles []manifest.Role, roleNames []string) []*spec.RoleV2 {
	var matchingRoles []*spec.RoleV2

	for _, roleName := range roleNames {
		for _, role := range roles {
			if role.Name == roleName {
				var roleType spec.RoleTypeV2

				// The manifest validation is handling the check if the target nodepools of the
				// role are control nodepools and thus can be used as a valid API loadbalancer.
				// Given this invariant we can simply check for the port.
				if role.TargetPort == manifest.APIServerPort {
					roleType = spec.RoleTypeV2_ApiServer_V2
				} else {
					roleType = spec.RoleTypeV2_Ingress_V2
				}

				if role.Settings == nil {
					role.Settings = &manifest.RoleSettings{
						ProxyProtocol: true,
					}
				}

				newRole := &spec.RoleV2{
					Name:        role.Name,
					Protocol:    strings.ToLower(role.Protocol),
					Port:        role.Port,
					TargetPort:  role.TargetPort,
					TargetPools: role.TargetPools,
					RoleType:    roleType,
					Settings: &spec.RoleV2_Settings{
						ProxyProtocol:  role.Settings.ProxyProtocol,
						StickySessions: role.Settings.StickySessions,
						// initially set as an invalid port, must be updated
						// later, when merging with the existing state to avoid
						// port duplication.
						EnvoyAdminPort: -1,
					},
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}

	return matchingRoles
}

func GenerateMissingCIDR(current, desired *spec.ClustersV2) error {
	// https://github.com/berops/claudie/issues/647
	// 1. generate cidrs for k8s nodepools.
	existing := make(map[string][]string)
	for p, nps := range nodepools.ByProviderRegion(current.GetK8S().GetClusterInfo().GetNodePools()) {
		for _, np := range nodepools.ExtractDynamic(nps) {
			existing[p] = append(existing[p], np.Cidr)
		}
	}

	for p, nps := range nodepools.ByProviderRegion(desired.GetK8S().GetClusterInfo().GetNodePools()) {
		if err := calculateCIDR(baseSubnetCIDR, p, existing, nodepools.ExtractDynamic(nps)); err != nil {
			return fmt.Errorf("error while generating cidr for nodepool: %w", err)
		}
	}

	// 2. generate cidrs for each lb nodepool
	for _, desired := range desired.GetLoadBalancers().GetClusters() {
		existing := make(map[string][]string)
		if i := clusters.IndexLoadbalancerByIdV2(desired.GetClusterInfo().Id(), current.GetLoadBalancers().GetClusters()); i >= 0 {
			current := current.GetLoadBalancers().GetClusters()[i]
			for p, nps := range nodepools.ByProviderRegion(current.GetClusterInfo().GetNodePools()) {
				for _, np := range nodepools.ExtractDynamic(nps) {
					existing[p] = append(existing[p], np.Cidr)
				}
			}
		}
		for p, nps := range nodepools.ByProviderRegion(desired.GetClusterInfo().GetNodePools()) {
			if err := calculateCIDR(baseSubnetCIDR, p, existing, nodepools.ExtractDynamic(nps)); err != nil {
				return fmt.Errorf("error while generating cidr for loadbalancer %q, nodepools: %w", desired.GetClusterInfo().Id(), err)
			}
		}
	}
	return nil
}

// calculateCIDR will make sure all nodepools have subnet CIDR calculated.
func calculateCIDR(baseCIDR, key string, existing map[string][]string, nodepools []*spec.DynamicNodePool) error {
	for _, np := range nodepools {
		if np.Cidr != "" {
			continue
		}

		cidr, err := getCIDR(baseCIDR, defaultOctetToChange, existing[key])
		if err != nil {
			return fmt.Errorf("failed to parse CIDR for nodepool : %w", err)
		}

		log.Debug().Msgf("Calculating new VPC subnet CIDR for nodepool. New CIDR [%s]", cidr)
		np.Cidr = cidr
		existing[key] = append(existing[key], cidr)
	}

	return nil
}

// getCIDR function returns CIDR in IPv4 format, with position replaced by value
// The function does not check if it is a valid CIDR/can be used in subnet spec
func getCIDR(baseCIDR string, position int, existing []string) (string, error) {
	_, ipNet, err := net.ParseCIDR(baseCIDR)
	if err != nil {
		return "", fmt.Errorf("cannot parse a CIDR with base %s, position %d", baseCIDR, position)
	}
	ip := ipNet.IP
	ones, _ := ipNet.Mask.Size()
	var i int
	for {
		if i > 255 {
			return "", fmt.Errorf("maximum number of IPs assigned")
		}
		ip[position] = byte(i)
		if slices.Contains(existing, fmt.Sprintf("%s/%d", ip.String(), ones)) {
			i++
			continue
		}
		return fmt.Sprintf("%s/%d", ip.String(), ones), nil
	}
}
