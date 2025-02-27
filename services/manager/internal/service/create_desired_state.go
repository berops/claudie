package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"

	"google.golang.org/protobuf/proto"

	"golang.org/x/crypto/ssh"
)

const (
	// CIDR used when creating the nodepools for desired state.
	baseSubnetCIDR       = "10.0.0.0/24"
	defaultOctetToChange = 2
)

func createDesiredState(pending *store.Config) error {
	// 1. If the infrastructure was marked for deletion the InputManifest and the ManifestChecksum will be empty.
	markedForDeletion := pending.Manifest.Raw == "" &&
		pending.Manifest.Checksum == nil &&
		pending.Manifest.State == manifest.Pending.String()

	if markedForDeletion {
		for cluster := range pending.Clusters {
			pending.Clusters[cluster].Desired.K8s = nil
			pending.Clusters[cluster].Desired.LoadBalancers = nil
		}
		return nil
	}

	// In the next steps It might be the case either the Current State or Desired state is nil and thus
	// these cases needs to be handled gracefully.

	// 2. create the desired state from the input manifest.
	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(pending.Manifest.Raw), &m); err != nil {
		return fmt.Errorf("error unmarshalling manifest for config %q: %w", pending.Name, err)
	}
	if err := createK8sClustersFromManifest(&m, pending); err != nil {
		return fmt.Errorf("failed to parse k8s clusters from manifest %q: %w", m.Name, err)
	}
	if err := createLBClustersFromManifest(&m, pending); err != nil {
		return fmt.Errorf("failed to parse lb clusters from manifest: %q: %w", m.Name, err)
	}

	grpcRepr, err := store.ConvertToGRPC(pending)
	if err != nil {
		return fmt.Errorf("failed to convert from db representation to grpc %q: %w", pending.Name, err)
	}

	// 2.1 Also consider to re-use existing data created in previous run if not first-run of the workflow for the manifest.
	for _, state := range grpcRepr.Clusters {
		deduplicateNodepoolNames(&m, state)
	}

	backwardsCompatiblity(grpcRepr)

	if err := transferExistingState(grpcRepr); err != nil {
		return fmt.Errorf("failed to reuse current state date for desired state for %q: %w", m.Name, err)
	}
	// after transferring existing state fill remaining data.
	for _, state := range grpcRepr.Clusters {
		fillMissingDynamicNodes(state.Desired)
	}

	// We generate the CIDR for individual nodepools at this step, as
	// we need contextual information about the current state so that
	// we do not generate the same cidr for the same Provider/Region pair
	// multiple times, avoiding conflicts.
	for _, state := range grpcRepr.Clusters {
		if err := fillMissingCIDR(state); err != nil {
			return fmt.Errorf("failed to generate cidrs for nodepools: %w", err)
		}
	}

	modified, err := store.ConvertFromGRPC(grpcRepr)
	if err != nil {
		return fmt.Errorf("failed to convert from grpc to db representation %q: %w", grpcRepr.Name, err)
	}

	*pending = *modified

	return nil
}

// createK8sClustersFromManifest parses manifest and creates desired state for Kubernetes clusters.
func createK8sClustersFromManifest(from *manifest.Manifest, into *store.Config) error {
	// 1. traverse all clusters in the manifest
	//    catching newly created or existing (updated).
	desiredClusters := make(map[string]struct{})
	for _, cluster := range from.Kubernetes.Clusters {
		useInstallationProxy := &spec.InstallationProxy{
			Mode: "default",
		}

		if cluster.InstallationProxy != nil {
			useInstallationProxy.Mode = cluster.InstallationProxy.Mode
			useInstallationProxy.Endpoint = cluster.InstallationProxy.Endpoint
		}

		newCluster := &spec.K8Scluster{
			ClusterInfo: &spec.ClusterInfo{
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

		// NOTE: we do not populate nodepool.CIDR at this stage

		if err := generateSSHKeys(newCluster.ClusterInfo); err != nil {
			return fmt.Errorf("error encountered while creating desired state for %s : %w", newCluster.ClusterInfo.Name, err)
		}

		clusterBytes, err := proto.Marshal(newCluster)
		if err != nil {
			return fmt.Errorf("failed to marshal k8s cluster %q: %w", newCluster.GetClusterInfo().Id(), err)
		}

		// It is guaranteed by validation, that within a single InputManifest no two clusters (including LB)
		// can share the same name.
		if v, ok := into.Clusters[newCluster.ClusterInfo.Name]; ok {
			v.Desired.K8s = clusterBytes
		} else {
			if into.Clusters == nil {
				into.Clusters = make(map[string]*store.ClusterState)
			}
			into.Clusters[newCluster.ClusterInfo.Name] = &store.ClusterState{Desired: store.Clusters{K8s: clusterBytes}}
		}

		desiredClusters[newCluster.ClusterInfo.Name] = struct{}{}
	}

	// 2. Catch clusters that were deleted.
	for clusterName := range into.Clusters {
		if _, ok := desiredClusters[clusterName]; ok {
			continue
		}

		// Mark for deletion.
		into.Clusters[clusterName].Desired.K8s = nil
		into.Clusters[clusterName].Desired.LoadBalancers = nil
	}

	return nil
}

// createLBClustersFromManifest reads the manifest and creates load balancer clusters based on it.
func createLBClustersFromManifest(from *manifest.Manifest, into *store.Config) error {
	// 1. Collect all Lbs in the desired state for given K8s clusters.
	lbs := make(map[string]*spec.LoadBalancers)
	for _, lbCluster := range from.LoadBalancer.Clusters {
		dns, err := getDNS(lbCluster.DNS, from)
		if err != nil {
			return fmt.Errorf("error while building desired state for LB %s : %w", lbCluster.Name, err)
		}

		attachedRoles := getRolesAttachedToLBCluster(from.LoadBalancer.Roles, lbCluster.Roles)

		newLbCluster := &spec.LBcluster{
			ClusterInfo: &spec.ClusterInfo{
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

		// NOTE: we do not populate nodepool.CIDR at this stage

		if err := generateSSHKeys(newLbCluster.ClusterInfo); err != nil {
			return fmt.Errorf("error encountered while creating desired state for %s : %w", newLbCluster.ClusterInfo.Name, err)
		}

		// delay the creation of the hostname at a later point
		// where we can re-use the current state.

		if _, ok := lbs[newLbCluster.TargetedK8S]; !ok {
			lbs[newLbCluster.TargetedK8S] = new(spec.LoadBalancers)
		}
		lbs[newLbCluster.TargetedK8S].Clusters = append(lbs[newLbCluster.TargetedK8S].Clusters, newLbCluster)
	}

	// 2. Marshal and match with respective clusters.
	for k8sCluster := range into.Clusters {
		lbs, ok := lbs[k8sCluster]
		if !ok {
			into.Clusters[k8sCluster].Desired.LoadBalancers = nil
			continue
		}

		bytes, err := proto.Marshal(lbs)
		if err != nil {
			return fmt.Errorf("failed to marshal lb clusters for %q: %w", k8sCluster, err)
		}

		into.Clusters[k8sCluster].Desired.LoadBalancers = bytes
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

	return &spec.DNS{DnsZone: dns.DNSZone, Provider: provider, Hostname: dns.Hostname}, nil
}

// getRolesAttachedToLBCluster will read roles attached to the LB cluster from the unmarshalled manifest and return them.
// Returns slice of *[]pb.Roles if successful, error if Target from manifest state not found
func getRolesAttachedToLBCluster(roles []manifest.Role, roleNames []string) []*spec.Role {
	var matchingRoles []*spec.Role

	for _, roleName := range roleNames {
		for _, role := range roles {
			if role.Name == roleName {
				var roleType spec.RoleType

				// The manifest validation is handling the check if the target nodepools of the
				// role are control nodepools and thus can be used as a valid API loadbalancer.
				// Given this invariant we can simply check for the port.
				if role.TargetPort == manifest.APIServerPort {
					roleType = spec.RoleType_ApiServer
				} else {
					roleType = spec.RoleType_Ingress
				}

				newRole := &spec.Role{
					Name:        role.Name,
					Protocol:    role.Protocol,
					Port:        role.Port,
					TargetPort:  role.TargetPort,
					TargetPools: role.TargetPools,
					RoleType:    roleType,
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}

	return matchingRoles
}

// generateSSHKeys will generate SSH keypair for each nodepool that does not yet have
// a keypair assigned.
func generateSSHKeys(desiredInfo *spec.ClusterInfo) error {
	for i := range desiredInfo.NodePools {
		if dp := desiredInfo.NodePools[i].GetDynamicNodePool(); dp != nil && dp.PublicKey == "" {
			var err error
			if dp.PublicKey, dp.PrivateKey, err = generateSSHKeyPair(); err != nil {
				return fmt.Errorf("error while create SSH key pair for nodepool %s: %w", desiredInfo.NodePools[i].Name, err)
			}
		}
	}
	return nil
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// Generate and write public key
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	b := &bytes.Buffer{}
	b.Write(ssh.MarshalAuthorizedKey(pubKey))
	b.Truncate(b.Len() - 1) // remove generated newline

	return b.String(), privKeyBuf.String(), nil
}

func fillMissingDynamicNodes(c *spec.Clusters) {
	k8sID := c.GetK8S().GetClusterInfo().Id()

	for _, np := range c.GetK8S().GetClusterInfo().GetNodePools() {
		if np.GetDynamicNodePool() == nil {
			continue
		}
		usedNames := make(map[string]struct{})
		for _, node := range np.Nodes {
			usedNames[node.Name] = struct{}{}
		}

		nodepoolID := fmt.Sprintf("%s-%s", k8sID, np.Name)
		generateMissingDynamicNodes(nodepoolID, usedNames, np)
	}

	for _, lb := range c.GetLoadBalancers().GetClusters() {
		lbID := lb.ClusterInfo.Id()
		for _, np := range lb.GetClusterInfo().GetNodePools() {
			if np.GetDynamicNodePool() == nil {
				continue
			}
			usedNames := make(map[string]struct{})
			for _, node := range np.Nodes {
				usedNames[node.Name] = struct{}{}
			}
			nodepoolID := fmt.Sprintf("%s-%s", lbID, np.Name)
			generateMissingDynamicNodes(nodepoolID, usedNames, np)
		}
	}
}

func generateMissingDynamicNodes(nodepoolID string, usedNames map[string]struct{}, np *spec.NodePool) {
	typ := spec.NodeType_worker
	if np.IsControl {
		typ = spec.NodeType_master
	}

	for len(np.Nodes) < int(np.GetDynamicNodePool().Count) {
		name := uniqueNodeName(nodepoolID, usedNames)
		usedNames[name] = struct{}{}
		np.Nodes = append(np.Nodes, &spec.Node{
			Name:     name,
			NodeType: typ,
		})
		log.Debug().Str("nodepool", nodepoolID).Msgf("adding new node %q into desired state IsControl: %v", name, np.IsControl)
	}
}

func fillMissingCIDR(c *spec.ClusterState) error {
	// https://github.com/berops/claudie/issues/647
	// 1. generate cidrs for k8s nodepools.
	existing := make(map[string][]string)
	for p, nps := range nodepools.ByProviderRegion(c.GetCurrent().GetK8S().GetClusterInfo().GetNodePools()) {
		for _, np := range nodepools.ExtractDynamic(nps) {
			existing[p] = append(existing[p], np.Cidr)
		}
	}

	for p, nps := range nodepools.ByProviderRegion(c.GetDesired().GetK8S().GetClusterInfo().GetNodePools()) {
		if err := calculateCIDR(baseSubnetCIDR, p, existing, nodepools.ExtractDynamic(nps)); err != nil {
			return fmt.Errorf("error while generating cidr for nodepool: %w", err)
		}
	}

	// 2. generate cidrs for each lb nodepool
	for _, desired := range c.GetDesired().GetLoadBalancers().GetClusters() {
		existing := make(map[string][]string)
		if i := clusters.IndexLoadbalancerById(desired.GetClusterInfo().Id(), c.GetCurrent().GetLoadBalancers().GetClusters()); i >= 0 {
			current := c.GetCurrent().GetLoadBalancers().GetClusters()[i]
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
