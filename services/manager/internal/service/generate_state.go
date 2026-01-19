package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// Length of the hash that is used for generating hostnames for nodepools that do not
// have one assigned in the parsed [manifest.Manifest].
const HostnameHashLength = 17

// PopulateDnsHostName generates a random [spec.Dns.Hostname] for all [spec.Clusters.LoadBalancers]
// that have their [spec.Dns.Hostname] empty. The generated hostname is a random sequence of length
// [HostnameHashLength]
func PopulateDnsHostName(state *spec.Clusters) {
	for _, lb := range state.GetLoadBalancers().GetClusters() {
		if lb.Dns.Hostname == "" {
			lb.Dns.Hostname = hash.Create(HostnameHashLength)
		}
	}
}

// PopulateSSHKeys will generate SSH keypair for each nodepool that does not yet have a keypair assigned.
func PopulateSSHKeys(state *spec.Clusters) error {
	for _, np := range state.GetK8S().GetClusterInfo().GetNodePools() {
		if err := generateSSHKeys(np); err != nil {
			return err
		}
	}

	for _, lb := range state.GetLoadBalancers().GetClusters() {
		for _, np := range lb.GetClusterInfo().GetNodePools() {
			if err := generateSSHKeys(np); err != nil {
				return err
			}
		}
	}

	return nil
}

// For each [spec.Clusters.Loadbalancers] adds a role with the [manifest.HealthcheckPort]
// if it already is not present.
func PopulateDefaultHealthcheckRole(state *spec.Clusters) {
	for _, lb := range state.GetLoadBalancers().GetClusters() {
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

// For each [spec.Clusters.LoadBalancers] generates a port for the
// envoy admin interface. The port is used from the claudie reserved range.
func PopulateEnvoyAdminPorts(state *spec.Clusters) {
	for _, lb := range state.GetLoadBalancers().GetClusters() {
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

// Same as [PopulateDynamicNodes] but goes over all of the clusters in `c`.
func PopulateDynamicNodesForClusters(c *spec.Clusters) {
	k8sID := c.GetK8S().GetClusterInfo().Id()
	for _, np := range c.GetK8S().GetClusterInfo().GetNodePools() {
		if np.GetDynamicNodePool() == nil {
			continue
		}
		PopulateDynamicNodes(k8sID, np)
	}

	for _, lb := range c.GetLoadBalancers().GetClusters() {
		lbID := lb.ClusterInfo.Id()
		for _, np := range lb.GetClusterInfo().GetNodePools() {
			if np.GetDynamicNodePool() == nil {
				continue
			}
			PopulateDynamicNodes(lbID, np)
		}
	}
}

// Based on the `len(nodepool.Nodes)` and the [spec.Dynamic_NodePool.Count]
// generates missing Dynamic Node entries suchs that all nodes within that nodepool
// will have unique names assigned to them. If the passed in `nodepool` is not dynamic
// the function panics.
func PopulateDynamicNodes(clusterID string, nodepool *spec.NodePool) {
	dnp := nodepool.GetDynamicNodePool()

	names := make(map[string]struct{}, dnp.Count)
	for _, n := range nodepool.Nodes {
		names[n.Name] = struct{}{}
	}

	typ := spec.NodeType_worker
	if nodepool.IsControl {
		typ = spec.NodeType_master
	}

	nodepoolID := fmt.Sprintf("%s-%s", clusterID, nodepool.Name)
	for len(nodepool.Nodes) < int(dnp.Count) {
		next := uniqueNodeName(nodepoolID, names)
		nodepool.Nodes = append(nodepool.Nodes, &spec.Node{
			Name:     next,
			NodeType: typ,
			Status:   spec.NodeStatus_Preparing,
		})

		log.
			Debug().
			Str("cluster", clusterID).
			Msgf("adding node %q into nodepool %q, isControl: %v", next, nodepool.Name, nodepool.IsControl)
	}
}

// uniqueNodeName returns a node name, which is guaranteed to be unique, based on the provided existing names.
// If the number of nodes exceed the supported amount of 255 the function panics. The newly generated node that
// that is returned is also stored in the passed in `existingNames` map.
func uniqueNodeName(nodepoolID string, existingNames map[string]struct{}) string {
	if len(existingNames)+1 > math.MaxUint8 {
		// The limit of [math.MaxUint8] is used due to the limitation of length
		// of node names. Various cloud providers or names in kuberentes itself
		// have length limits.
		panic("requesting to generate more nodes than is internally supported")
	}

	index := uint8(1)
	for {
		candidate := fmt.Sprintf("%s-%02x", nodepoolID, index)
		if _, ok := existingNames[candidate]; !ok {
			existingNames[candidate] = struct{}{}
			return candidate
		}
		index++
	}
}

func generateSSHKeys(np *spec.NodePool) error {
	if dp := np.GetDynamicNodePool(); dp != nil && dp.PublicKey == "" {
		var err error
		if dp.PublicKey, dp.PrivateKey, err = generateSSHKeyPair(); err != nil {
			return fmt.Errorf("error while create SSH key pair for nodepool %s: %w", np.Name, err)
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
