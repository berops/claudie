package service

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
)

// Port on which the ssh connection is established when healthchecking
// individual nodes.
const sshPort = "22"

// TODO: check if in terraform it is preferred to use the cached
// local version instead of downloading the templates.

// UnreachableNodesMap holds the nodepools and all of the nodes within
// that nodepool that are unreachable via a Ping on the IPv4 public endpoint.
type UnreachableIPv4Map = map[string][]string

// HealthCheckStatus report the status of the Infrastructure.
type HealthCheckStatus struct {
	// HealthCheck on the 6443 port, which is used for the API server
	// for the kubernetes cluster.
	ApiEndpoint struct {
		// Whether the API endpoint could be reached. The check is done
		// by trying to reach the cluster via `kubectl`for the [spec.K8Scluster.Kubeconfig].
		//
		// NOTE: This does not necessarily mean the cluster itself is down
		// it could be the case that the managemenet cluster has network issues
		// and the packets could not reach the cluster.
		Unreachable bool
	}

	// Describes unreachable nodes across a [spec.Clusters].
	UnreachableNodes struct {
		// Nodepools and their unreachable nodes in the kubernetes cluster.
		Kubernetes UnreachableIPv4Map

		// Loadbalancers attached to the kubernetes cluster and for each
		// of them Nodepools with the unreachable node within each.
		LoadBalancers map[string]UnreachableIPv4Map
	}

	Cluster struct {
		// Nodes, as returned by kubectl get nodes.
		Nodes map[string]struct{}

		// Whether the Port 6443 is exported on the control nodes.
		ControlNodesHave6443 bool

		// Whether there is a drift in the number of wireguard peers.
		VpnDrift bool
	}
}

// HealthCheck performs healthcheck across the passed in [spec.Clusters] state.
func HealthCheck(logger zerolog.Logger, state *spec.ClustersV2) (HealthCheckStatus, error) {
	var result HealthCheckStatus
	result.Cluster.Nodes = make(map[string]struct{})

	k, lb, err := clusters.PingNodesV2(logger, state)
	if err != nil {
		if !errors.Is(err, clusters.ErrEchoTimeout) {
			logger.Err(err).Msg("failed to determine if any nodes were unreachable")
			// Return error here, as the Pinging could fail due to permissions issues
			// in which case the healthcheck cannot be interpreted properly.
			return result, err
		}
		// If there is a [clusters.ErrEchoTimeout], fallthrough as the
		// returned k, lb values will have unreachable nodes.
	}

	result.UnreachableNodes.Kubernetes = k
	result.UnreachableNodes.LoadBalancers = lb

	kc := kubectl.Kubectl{
		MaxKubectlRetries: -1,
	}

	logger.Info().Msg("Verifying if Api server is reachable")

	n, err := kc.KubectlGetNodeNames()
	if err != nil {
		// Does not necessarily mean the cluster is down
		// the management cluster could have network issues.
		result.ApiEndpoint.Unreachable = true
	}

	for node := range strings.SplitSeq(string(n), "\n") {
		result.Cluster.Nodes[node] = struct{}{}
	}

	// Test a random control node if the 6443 port is reachable.
	controlpools := nodepools.Control(state.K8S.ClusterInfo.NodePools)
	if node := nodepools.RandomNode(controlpools); node != nil {
		endpoint := net.JoinHostPort(node.Public, fmt.Sprint(manifest.APIServerPort))
		if c, err := net.DialTimeout("tcp", endpoint, clusters.PingTimeout); err == nil {
			result.Cluster.ControlNodesHave6443 = true
			c.Close()
		}
	}

	ok, err := healthCheckVPN(state)
	if err != nil {
		// This should only be hit on connection issues.
		// The reason for this is that, if the management cluster
		// cannot connect to the node, it does not necessarily mean
		// that there is a drift. To avoid scheduling a false positive
		// simply assume its okay.
		logger.
			Warn().
			Msgf("failed to health check peers of a node within the cluster: %v, assuming okay", err)
		ok = true
	}

	result.Cluster.VpnDrift = !ok

	return result, nil
}

// HealthChecks the number of Peers of the wireguard network of a random Node.
// On any error with connecting to the node a non-nil error is returned, otherwise
// there will always be a non-nil error. If the peers match the expected number true
// is returned otherwise false.
func healthCheckVPN(state *spec.ClustersV2) (bool, error) {
	nps := state.K8S.ClusterInfo.NodePools
	for _, lb := range state.LoadBalancers.Clusters {
		nps = append(nps, lb.ClusterInfo.NodePools...)
	}

	username, public, key := nodepools.RandomNodePublicEndpoint(nps)
	if key == "" {
		// If there is no key, than there is no node. assume all is okay.
		return true, nil
	}

	signer, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return false, fmt.Errorf("node has an invalid private key: %w", err)
	}

	cfg := ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),

		// A single ping can take up to ~330ms across
		// the globe, 2 seconds should be generous for
		// establishing the TCP connection.
		Timeout: 2 * time.Second,
	}

	endpoint := net.JoinHostPort(public, sshPort)
	client, err := ssh.Dial("tcp", endpoint, &cfg)
	if err != nil {
		return false, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return false, err
	}
	defer session.Close()

	// According to the documentation the output of wiregaurd is safe to be
	// parsed, even within scripts
	// https://manpages.debian.org/unstable/wireguard-tools/wg.8.en.html#show
	b, err := session.Output("wg show all peers")
	if err != nil {
		return false, err
	}

	output := strings.TrimSpace(string(b))
	peers := 0
	for _, p := range slices.Collect(strings.SplitSeq(output, "\n")) {
		// Claudie creates/expectets the peers to be connected on the
		// wg0 interface, see ansible-playbook templates for configuring
		// wiregaurd.
		if strings.Contains(p, "wg0") {
			peers += 1
		}
	}
	nodeCount := nodepools.NodeCount(nps)

	// exempt the connect to node.
	return peers == nodeCount-1, nil
}
