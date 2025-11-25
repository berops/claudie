package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
)

func backwardsCompatiblityTransferMissingState(c *spec.Config) {
	for _, state := range c.GetClusters() {
		for _, current := range state.GetCurrent().GetLoadBalancers().GetClusters() {
			// TODO: remove in future versions, cloudflare account id may not be correctly
			// propagated to the current state when upgrading claudie versions, since the
			// [manifest.Cloudflare.AccountID] has a validation which requires the presence
			// of a valid, non-empty `account_id`, which might be missing in the current state,
			// that will result in errors on subsequent workflows, simply transfer the `account_id`
			// from the desired state to the current state, only if it's empty. That will take care
			// of the drift introduced during claudie updates.
			if cc := current.GetDns().GetProvider().GetCloudflare(); cc != nil && cc.AccountID == "" {
				i := clusters.IndexLoadbalancerById(current.GetClusterInfo().Id(), state.GetDesired().GetLoadBalancers().GetClusters())
				if i >= 0 {
					dlb := state.Desired.LoadBalancers.Clusters[i]
					if dc := dlb.GetDns().GetProvider().GetCloudflare(); dc != nil {
						log.
							Info().
							Str("cluster", current.GetClusterInfo().Id()).
							Msg("detected drift in current state for Cloudflare AccountID, transfering state from desired state")
						cc.AccountID = dc.AccountID
					}
				}
			}
		}
	}
}

func backwardsCompatibility(c *spec.Config) {
	// TODO: remove in future versions, currently only for backwards compatibility.
	// version 0.9.3 moved selection of the api server to the manager service
	// and introduced a new field that selects which LB is used as the api endpoint.
	// To have backwards compatibility with clusters build with versions before 0.9.3
	// select the first load balancer in the current state and set this new field to true.
	for _, state := range c.GetClusters() {
		currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()
		var (
			anyApiServerLoadBalancerSelected bool
			apiServerLoadBalancers           []int
		)

		for _, current := range currentLbs {
			// TODO: remove in future versions, currently only for backwards compatibility.
			// version 0.9.7 introced additional role settings, which may not be set in the
			// current state. To have backwards compatibility add defaults to the current state.
			for _, role := range current.Roles {
				if role.Settings == nil {
					log.Info().
						Str("cluster", current.GetClusterInfo().Id()).
						Msg("detected loadbalancer build with version older than 0.9.7, settings default role settings for its current state")

					role.Settings = &spec.Role_Settings{
						ProxyProtocol: true,
					}
				}
			}
		}

		for i, current := range currentLbs {
			if current.IsApiEndpoint() {
				anyApiServerLoadBalancerSelected = true
				break
			}
			if current.HasApiRole() && !current.UsedApiEndpoint && current.Dns != nil {
				apiServerLoadBalancers = append(apiServerLoadBalancers, i)
			}
		}
		if !anyApiServerLoadBalancerSelected && len(apiServerLoadBalancers) > 0 {
			currentLbs[apiServerLoadBalancers[0]].UsedApiEndpoint = true
			log.Info().
				Str("cluster", currentLbs[apiServerLoadBalancers[0]].GetClusterInfo().Id()).
				Msgf("detected api-server loadbalancer build with claudie version older than 0.9.3, selecting as the loadbalancer for the api-server")
		}
	}
}

// transferExistingState transfers existing data from current state to desired.
func transferExistingState(c *spec.Config) error {
	for cluster, state := range c.GetClusters() {
		log.Debug().Str("cluster", cluster).Msgf("reusing existing state")

		if err := transferExistingK8sState(state.GetCurrent().GetK8S(), state.GetDesired().GetK8S()); err != nil {
			return fmt.Errorf("error while updating Kubernetes cluster %q for config %s : %w", cluster, c.Name, err)
		}

		if err := transferExistingLBState(state.GetCurrent().GetLoadBalancers(), state.GetDesired().GetLoadBalancers()); err != nil {
			return fmt.Errorf("error while updating Loadbalancer cluster %q for config %s : %w", cluster, c.Name, err)
		}
	}

	return nil
}

// deduplicateNodepoolNames renames multiple references of the same nodepool in k8s,lb clusters to treat
// them as individual nodepools.
func deduplicateNodepoolNames(from *manifest.Manifest, state *spec.ClusterState) {
	desired := state.GetDesired().GetK8S()
	desiredLbs := state.GetDesired().GetLoadBalancers().GetClusters()

	current := state.GetCurrent().GetK8S()
	currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()

	for _, np := range from.NodePools.Dynamic {
		used := make(map[string]struct{})

		copyK8sNodePoolsNamesFromCurrentState(used, np.Name, current, desired)
		copyLbNodePoolNamesFromCurrentState(used, np.Name, currentLbs, desiredLbs)

		references := findNodePoolReferences(np.Name, desired.GetClusterInfo().GetNodePools())
		for _, lb := range desiredLbs {
			references = append(references, findNodePoolReferences(np.Name, lb.GetClusterInfo().GetNodePools())...)
		}

		for _, np := range references {
			h := hash.Create(hash.Length)
			for _, ok := used[h]; ok; {
				h = hash.Create(hash.Length)
			}
			used[h] = struct{}{}
			np.Name += fmt.Sprintf("-%s", h)
		}
	}
}

// copyLbNodePoolNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyLbNodePoolNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired []*spec.LBcluster) {
	for _, desired := range desired {
		references := findNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())
		switch {
		case len(references) > 1:
			panic("unexpected nodepool reference count")
		case len(references) == 0:
			continue
		}

		ref := references[0]

		for _, current := range current {
			if desired.ClusterInfo.Name != current.ClusterInfo.Name {
				continue
			}

			for _, np := range current.GetClusterInfo().GetNodePools() {
				_, hash := nodepools.MatchNameAndHashWithTemplate(nodepool, np.Name)
				if hash == "" {
					continue
				}

				used[hash] = struct{}{}

				ref.Name += fmt.Sprintf("-%s", hash)
				break
			}
		}
	}
}

// copyK8sNodePoolsNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyK8sNodePoolsNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired *spec.K8Scluster) {
	references := findNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())

	switch {
	case len(references) == 0:
		return
	case len(references) > 2:
		// cannot reuse the same nodepool more than twice (once for control, once for worker pools)
		panic("unexpected nodepool reference count")
	}

	// to avoid extra code for special cases where there is just 1 reference, append a nil.
	references = append(references, []*spec.NodePool{nil}...)

	control, compute := references[0], references[1]
	if !references[0].IsControl {
		control, compute = compute, control
	}

	for _, np := range current.GetClusterInfo().GetNodePools() {
		_, hash := nodepools.MatchNameAndHashWithTemplate(nodepool, np.Name)
		if hash == "" {
			continue
		}

		used[hash] = struct{}{}

		if np.IsControl && control != nil {
			// if there are multiple nodepools in the current state (for example on a failed rolling update)
			// transfer only one of them.
			if _, h := nodepools.MatchNameAndHashWithTemplate(nodepool, control.Name); h == "" {
				control.Name += fmt.Sprintf("-%s", hash)
			}
		} else if !np.IsControl && compute != nil {
			// if there are multiple nodepools in the current state (for example on a failed rolling update)
			// transfer only one of them.
			if _, h := nodepools.MatchNameAndHashWithTemplate(nodepool, compute.Name); h == "" {
				compute.Name += fmt.Sprintf("-%s", hash)
			}
		}
	}
}

// findNodePoolReferences find all nodepools that share the given name.
func findNodePoolReferences(name string, nodePools []*spec.NodePool) []*spec.NodePool {
	var references []*spec.NodePool
	for _, np := range nodePools {
		if np.Name == name {
			references = append(references, np)
		}
	}
	return references
}

// transferExistingK8sState updates the desired state of the kubernetes clusters based on the current state
func transferExistingK8sState(current, desired *spec.K8Scluster) error {
	if desired == nil || current == nil {
		return nil
	}

	if err := transferNodePools(desired.ClusterInfo, current.ClusterInfo); err != nil {
		return err
	}

	if current.Kubeconfig != "" {
		desired.Kubeconfig = current.Kubeconfig
	}

	return nil
}

// transferDynamicNp updates the desired state of the kubernetes clusters based on the current state
// transferring only nodepoolData.
func transferDynamicNp(clusterID string, current, desired *spec.NodePool, updateAutoscaler bool) bool {
	dnp := desired.GetDynamicNodePool()
	cnp := current.GetDynamicNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey
	dnp.Cidr = cnp.Cidr

	if updateAutoscaler && dnp.AutoscalerConfig != nil {
		switch {
		case dnp.AutoscalerConfig.Min > cnp.Count:
			dnp.Count = dnp.AutoscalerConfig.Min
		case dnp.AutoscalerConfig.Max < cnp.Count:
			dnp.Count = dnp.AutoscalerConfig.Max
		default:
			dnp.Count = cnp.Count
		}
	}

	fillDynamicNodes(clusterID, current, desired)
	return true
}

// updateClusterInfo updates the desired state based on the current state
// clusterInfo.
func transferNodePools(desired, current *spec.ClusterInfo) error {
	desired.Hash = current.Hash
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			if desiredNp.Name != currentNp.Name {
				continue
			}

			switch {
			case transferDynamicNp(desired.Id(), currentNp, desiredNp, true):
			case transferStaticNodes(desired.Id(), currentNp, desiredNp):
			default:
				return fmt.Errorf("%q is neither dynamic nor static, unexpected value: %T", desiredNp.Name, desiredNp.Type)
			}

			continue desired
		}
	}
	return nil
}

func fillDynamicNodes(clusterID string, current, desired *spec.NodePool) {
	dnp := desired.GetDynamicNodePool()

	nodes := make([]*spec.Node, 0, dnp.Count)
	nodeNames := make(map[string]struct{}, dnp.Count)

	for i, node := range current.Nodes {
		if i == int(dnp.Count) {
			break
		}
		nodes = append(nodes, node)
		nodeNames[node.Name] = struct{}{}
		log.Debug().Str("cluster", clusterID).Msgf("reusing node %q from current state nodepool %q, IsControl: %v, into desired state of the nodepool", node.Name, desired.Name, desired.IsControl)
	}

	typ := spec.NodeType_worker
	if desired.IsControl {
		typ = spec.NodeType_master
	}
	nodepoolID := fmt.Sprintf("%s-%s", clusterID, desired.Name)
	for len(nodes) < int(dnp.Count) {
		name := uniqueNodeName(nodepoolID, nodeNames)
		nodeNames[name] = struct{}{}
		nodes = append(nodes, &spec.Node{
			Name:     name,
			NodeType: typ,
		})
		log.Debug().Str("cluster", clusterID).Msgf("adding node %q into desired state nodepool %q, IsControl: %v", name, desired.Name, desired.IsControl)
	}

	desired.Nodes = nodes
}

// Generates the entire range of reserved ports, including the ports for static services
// like NodeExporter and Healthcheck. To exclude these ports you can re-slice the result
// using [manifest.MaxRolesPerLoadBalancer]
func generateClaudieReservedPorts() []int {
	size := manifest.ReservedPortRangeEnd - manifest.ReservedPortRangeStart
	p := make([]int, size)
	for i := range size {
		p[i] = manifest.ReservedPortRangeStart + i
	}
	return p
}

func fillMissingEnvoyAdminPorts(desired *spec.Clusters) {
	for _, lb := range desired.GetLoadBalancers().GetClusters() {
		used := make(map[int]struct{})
		for _, r := range lb.Roles {
			if r.Settings.EnvoyAdminPort >= 0 {
				used[int(r.Settings.EnvoyAdminPort)] = struct{}{}
			}
		}

		// The number of roles is limited to [manifest.MaxRolesPerLoadBalancer],
		// thus we will never consume all of the ports.
		freePorts := generateClaudieReservedPorts()[:manifest.MaxRolesPerLoadBalancer]
		if len(used) > 0 {
			freePorts = slices.DeleteFunc(freePorts, func(port int) bool {
				_, ok := used[port]
				return ok
			})
		}

		for _, r := range lb.Roles {
			if r.Settings.EnvoyAdminPort < 0 {
				p := freePorts[len(freePorts)-1]
				freePorts = freePorts[:len(freePorts)-1]
				r.Settings.EnvoyAdminPort = int32(p)
			}
		}
	}
}

func fillDefaultHealthcheckRole(desired *spec.Clusters) {
	for _, lb := range desired.GetLoadBalancers().GetClusters() {
		// as this function is called after merging the current state to the desired
		// state, existing clusters already could have the healthcheck created.
		healthcheck := func(r *spec.Role) bool { return r.Port == manifest.HealthcheckPort }
		if slices.ContainsFunc(lb.Roles, healthcheck) {
			continue
		}

		healthcheckRole := &spec.Role{
			Name:     "internal.claudie.healthcheck",
			Protocol: "tcp",
			Port:     manifest.HealthcheckPort,
			// This is not a valid target port number. The healthcheck role
			// is only used for TCP healthchecks using the 3-way handshake
			// on the loadbalancers. Thus settings the TargetPort to an
			// invalid number leaving the TargetPools empty will result
			// in the opening of the [manifest.HealthcheckPort] on the firewall
			// which will be forwarded to the loadbalancer nodes, but thats
			// where the packets will end as no further forwarding will be
			// done.
			TargetPort:  -1,
			TargetPools: []string{},
			RoleType:    spec.RoleType_Ingress,
			Settings: &spec.Role_Settings{
				ProxyProtocol:  false,
				StickySessions: false,
				EnvoyAdminPort: manifest.HealthcheckEnvoyPort,
			},
		}

		lb.Roles = append(lb.Roles, healthcheckRole)
	}
}

// uniqueNodeName returns new node name, which is guaranteed to be unique, based on the provided existing names.
func uniqueNodeName(nodepoolID string, existingNames map[string]struct{}) string {
	index := uint8(1)
	for {
		candidate := fmt.Sprintf("%s-%02x", nodepoolID, index)
		if _, ok := existingNames[candidate]; !ok {
			return candidate
		}
		index++
	}
}

func transferStaticNodes(clusterID string, current, desired *spec.NodePool) bool {
	dsp := desired.GetStaticNodePool()
	csp := current.GetStaticNodePool()

	canUpdate := dsp != nil && csp != nil
	if !canUpdate {
		return false
	}

	usedNames := make(map[string]struct{})
	for _, cn := range current.Nodes {
		usedNames[cn.Name] = struct{}{}
	}

	for _, dn := range desired.Nodes {
		i := slices.IndexFunc(current.Nodes, func(cn *spec.Node) bool { return cn.Public == dn.Public })
		if i < 0 {
			dn.Name = uniqueNodeName(desired.Name, usedNames)
			usedNames[dn.Name] = struct{}{}
			log.Debug().Str("cluster", clusterID).Msgf("adding static node %q into desired state nodepool %q, IsControl: %v", dn.Name, desired.Name, desired.IsControl)
			continue
		}
		dn.Name = current.Nodes[i].Name
		dn.Private = current.Nodes[i].Private
		dn.NodeType = current.Nodes[i].NodeType
		log.Debug().Str("cluster", clusterID).Msgf("reusing node %q from current state static nodepool %q, IsControl: %v, into desired state static nodepool", dn.Name, desired.Name, desired.IsControl)
	}

	return true
}

// transferExistingLBState updates the desired state of the loadbalancer clusters based on the current state
func transferExistingLBState(current, desired *spec.LoadBalancers) error {
	transferExistingDns(current, desired)

	currentLbs := current.GetClusters()
	desiredLbs := desired.GetClusters()

	for _, desired := range desiredLbs {
		for _, current := range currentLbs {
			if current.ClusterInfo.Name != desired.ClusterInfo.Name {
				continue
			}

			if err := transferNodePools(desired.ClusterInfo, current.ClusterInfo); err != nil {
				return err
			}

			transferExistingRoles(current.Roles, desired.Roles)
			desired.UsedApiEndpoint = current.UsedApiEndpoint
			break
		}
	}

	return nil
}

func transferExistingRoles(current, desired []*spec.Role) {
	currentRoles := make(map[string]*spec.Role) // role names are unique
	for _, r := range current {
		currentRoles[r.Name] = r
	}

	for _, r := range desired {
		if prev, ok := currentRoles[r.Name]; ok {
			r.Settings.EnvoyAdminPort = prev.Settings.EnvoyAdminPort
		}
	}
}

func transferExistingDns(current, desired *spec.LoadBalancers) {
	const hostnameHashLength = 17

	currentLbs := make(map[string]*spec.LBcluster)
	for _, cluster := range current.GetClusters() {
		currentLbs[cluster.ClusterInfo.Name] = cluster
	}

	for _, cluster := range desired.GetClusters() {
		previous, ok := currentLbs[cluster.ClusterInfo.Name]
		// check if lb cluster in current state exists and was build successfully.
		if !ok || previous.Dns == nil {
			if cluster.Dns.Hostname == "" {
				cluster.Dns.Hostname = hash.Create(hostnameHashLength)
			}
			continue
		}

		// transfer matching alternative names, if any.
		for _, prev := range previous.Dns.AlternativeNames {
			i := slices.IndexFunc(cluster.Dns.AlternativeNames, func(n *spec.AlternativeName) bool {
				return n.Hostname == prev.Hostname
			})
			if i < 0 {
				continue
			}
			cluster.Dns.AlternativeNames[i].Endpoint = prev.Endpoint
		}

		// transfer the endpoint if the hostname did not change.
		if cluster.Dns.Hostname != "" {
			if previous.Dns.Hostname == cluster.Dns.Hostname {
				cluster.Dns.Endpoint = previous.Dns.Endpoint
			}
			continue
		}

		// keep hostname from current state if not specified in manifest
		if cluster.Dns.Hostname == "" || cluster.Dns.Endpoint == "" {
			cluster.Dns.Hostname = previous.Dns.Hostname
			cluster.Dns.Endpoint = previous.Dns.Endpoint
		}
	}
}
