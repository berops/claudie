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

// UnreachableNodesMap holds the nodepools and all of the nodes within
// that nodepool that are unreachable via a Ping on the IPv4 public endpoint.
type UnreachableIPv4Map = map[string][]string

type UnreachableNodes struct {
	// Nodepools and their unreachable nodes in the kubernetes cluster.
	Kubernetes UnreachableIPv4Map

	// Loadbalancers attached to the kubernetes cluster and for each
	// of them Nodepools with the unreachable node within each.
	LoadBalancers map[string]UnreachableIPv4Map
}

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

	Cluster struct {
		// Nodes, as returned by kubectl get nodes.
		Nodes map[string]struct{}

		// Whether the Port 6443 is exported on the control nodes.
		ControlNodesHave6443 bool

		// Whether there is a drift in the number of wireguard peers.
		VpnDrift bool
	}
}

// HealthCheckNodeReachability performs healthches across the nodes of the passed in [spec.Clusters] state.
// If there are any issues with the pinging of the nodes a non-nil error is returned which can be interpreted
// as failing to fully determine if a node is reachable or not.
func HealthCheckNodeReachability(logger zerolog.Logger, state *spec.Clusters) (UnreachableNodes, error) {
	var (
		err error

		result = UnreachableNodes{
			Kubernetes:    make(UnreachableIPv4Map),
			LoadBalancers: make(map[string]UnreachableIPv4Map),
		}
	)

	result.Kubernetes, result.LoadBalancers, err = clusters.PingNodes(logger, state)
	if err != nil {
		if !errors.Is(err, clusters.ErrEchoTimeout) {
			logger.
				Err(err).
				Msg("Failed to determine if any nodes were unreachable")

			// Return error here, as the Pinging could fail due to permissions issues
			// in which case the healthcheck cannot be interpreted properly.
			return result, err
		}

		// If there is a [clusters.ErrEchoTimeout], fallthrough as the
		// returned k, lb values will have unreachable nodes.
	}

	return result, nil
}

// HealthCheck performs healthcheck across the kubernetes cluster of the passed in [spec.Clusters] state.
func HealthCheck(logger zerolog.Logger, state *spec.Clusters) HealthCheckStatus {
	var result HealthCheckStatus
	result.Cluster.Nodes = make(map[string]struct{})

	kc := kubectl.Kubectl{
		Kubeconfig:        state.K8S.Kubeconfig,
		MaxKubectlRetries: -1,
	}

	n, err := kc.KubectlGetNodeNames()
	if err != nil {
		logger.Warn().Msgf("Failed to retrieve nodes of the cluster via `kubectl`: %v", err)
		// Does not necessarily mean the cluster is down
		// the management cluster could have network issues.
		n = []byte{}
		result.ApiEndpoint.Unreachable = true
	}

	for node := range strings.SplitSeq(string(n), "\n") {
		if node := strings.TrimSpace(node); node != "" {
			result.Cluster.Nodes[node] = struct{}{}
		}
	}

	// Test a random dynamic control node if the 6443 port is reachable.
	// We can't test static nodes as we do not have the ability to control
	// them afterwards.
	//
	// This check is here for the reconciliation loop to know when to close
	// the 6443 if an LoadBalancer was attached.
	controlpools := nodepools.Control(state.K8S.ClusterInfo.NodePools)
	if node := nodepools.RandomDynamicNode(controlpools); node != nil {
		endpoint := net.JoinHostPort(node.Public, fmt.Sprint(manifest.APIServerPort))
		// nolint
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
	return result
}

// HealthChecks the number of Peers of the wireguard network of a random Node.
// On any error with connecting to the node a non-nil error is returned, otherwise
// there will always be a non-nil error. If the peers match the expected number true
// is returned otherwise false.
func healthCheckVPN(state *spec.Clusters) (bool, error) {
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
		return false, fmt.Errorf("node %q has an invalid private key: %w", public, err)
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

	// According to the documentation the output of wireguard is safe to be
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
		// wireguard.
		if strings.Contains(p, "wg0") {
			peers += 1
		}
	}
	nodeCount := nodepools.NodeCount(nps)

	// exempt the connected to node.
	return peers == nodeCount-1, nil
}
