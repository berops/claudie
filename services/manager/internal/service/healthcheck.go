package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeOutput struct {
	Kind string `json:"kind"`

	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`

	Status struct {
		Conditions []corev1.NodeCondition
	}
}

type NodeDescription struct {
	K8sName string
	Ready   bool

	IsStatic   bool
	NodePool   string
	PublicIPv4 string
	IsControl  bool

	// Could be omitted.
	// If absent, can be interpreted as unknown state.
	LastTransitionTime *metav1.Time
}

// UnreachableNodesMap holds the nodepools and all of the nodes within
// that nodepool that are unreachable via a Ping on the IPv4 public endpoint.
type UnreachableIPv4Map = map[string][]string

type UnknownNodeStatus struct {
	// Nodepools and their nodes in the kubernetes cluster with unknown status
	// that is read from the kubernetes API.
	//
	// This structure contains nodes that are:
	//  - nodes with no reachable endpoint.
	//  - present in the current state but not in the kubernetes cluster.
	//  - present in the kubernetes cluster but not in the current state
	//  - present in both but with Unknown status.
	//
	// If the Api server of the cluster is unreachable, this will be empty.
	UnknownKubernetesNodes map[string][]NodeDescription

	// Loadbalancers attached to the kubernetes cluster and for each
	// of them Nodepools with the nodes for which the pings have failed.
	UnknownLoadBalancersNodes map[string]UnreachableIPv4Map
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
		//
		// Note that some of the fields may be unset as
		// the current state by claudie could be different
		// than the actual nodes inside the kubernetes cluster.
		Nodes map[string]*NodeDescription

		// Whether the Port 6443 is exported on the control nodes.
		ControlNodesHave6443 bool

		// Whether there is a drift in the number of wireguard peers.
		VpnDrift bool
	}
}

// HealthCheckNodeReachability performs healthchecks across the nodes of the passed in [spec.Clusters] state.
// If there are any issues with the pinging of the nodes a non-nil error is returned which can be interpreted
// as failing to fully determine if a node is reachable or not.
func HealthCheckNodeReachability(
	logger zerolog.Logger,
	state *spec.Clusters,
	hc HealthCheckStatus,
) (UnknownNodeStatus, error) {
	var (
		err error

		result = UnknownNodeStatus{
			UnknownKubernetesNodes:    make(map[string][]NodeDescription),
			UnknownLoadBalancersNodes: make(map[string]UnreachableIPv4Map),
		}
	)

	// If the api server is not reachable by claudie, we can't really
	// make any assumptions about the reachability of the kubernetes nodes
	// and thus simply default to considering them all healthy.
	if !hc.ApiEndpoint.Unreachable {
		clusterNodes := maps.Clone(hc.Cluster.Nodes)

		for _, np := range state.K8S.ClusterInfo.NodePools {
			for _, n := range np.Nodes {
				// kubernetes names have stripped cluster prefix.
				strippedName := strings.TrimPrefix(n.Name, fmt.Sprintf("%s-", state.K8S.ClusterInfo.Id()))

				v, inCluster := clusterNodes[strippedName]

				// Delete it from the cluster Nodes, to know we have processed it.
				delete(clusterNodes, strippedName)

				if n.Public == "" {
					result.UnknownKubernetesNodes[np.Name] = append(result.UnknownKubernetesNodes[np.Name], NodeDescription{
						K8sName:            strippedName,
						Ready:              false,
						IsStatic:           np.GetStaticNodePool() != nil,
						NodePool:           np.Name,
						PublicIPv4:         "",
						IsControl:          np.IsControl,
						LastTransitionTime: nil,
					})

					continue
				}

				if inCluster {
					// node in the cluster.
					if !v.Ready {
						result.UnknownKubernetesNodes[v.NodePool] = append(result.UnknownKubernetesNodes[v.NodePool], NodeDescription{
							K8sName:            v.K8sName,
							Ready:              v.Ready,
							IsStatic:           v.IsStatic,
							NodePool:           v.NodePool,
							PublicIPv4:         v.PublicIPv4,
							IsControl:          v.IsControl,
							LastTransitionTime: v.LastTransitionTime.DeepCopy(),
						})
					}
				} else {
					// in current state but not in cluster.
					result.UnknownKubernetesNodes[np.Name] = append(result.UnknownKubernetesNodes[np.Name], NodeDescription{
						K8sName:            strippedName,
						Ready:              false,
						IsStatic:           np.GetStaticNodePool() != nil,
						NodePool:           np.Name,
						PublicIPv4:         n.Public,
						IsControl:          np.IsControl,
						LastTransitionTime: nil,
					})
				}
			}
		}

		// nodes that are in the k8s cluster but not in our tracked state.
		for _, n := range clusterNodes {
			result.UnknownKubernetesNodes[n.NodePool] = append(result.UnknownKubernetesNodes[n.NodePool], NodeDescription{
				K8sName: n.K8sName,
				Ready:   false, // since we do not track them, consider them as not ready.

				// dynamic nodes are explicitcly tracked by claudie
				// thus if there is a node in the k8s cluster and
				// not tracked by claudie it has to be a static node.
				IsStatic: true,

				NodePool:           n.NodePool,
				PublicIPv4:         n.PublicIPv4,
				IsControl:          n.IsControl,
				LastTransitionTime: nil,
			})
		}
	} else {
		logger.
			Warn().
			Msg("Api server unreachable, skip health-checking kubernetes nodes")
	}

	result.UnknownLoadBalancersNodes, err = clusters.PingLoadBalancerNodes(logger, state)
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
	result.Cluster.Nodes = make(map[string]*NodeDescription)

	kc := kubectl.Kubectl{
		Kubeconfig:        state.K8S.Kubeconfig,
		MaxKubectlRetries: -1,
	}

	out, err := kc.KubectlGet("nodes", "-ojson")
	if err != nil {
		logger.
			Warn().
			Msgf("Failed to retrieve nodes of the cluster via `kubectl`: %v", err)

		out = []byte{}
		result.ApiEndpoint.Unreachable = true
	}

	var description struct {
		Items []NodeOutput `json:"items"`
	}

	if err := json.Unmarshal(out, &description); err != nil {
		logger.
			Warn().
			Msgf("failed to unmarshal json output from kubectl get nodes, ignoring: %v", err)

		description.Items = nil
		result.ApiEndpoint.Unreachable = true
	}

	if len(description.Items) == 0 {
		// Does not necessarily mean the cluster is down
		// the management cluster could have network issues.
		result.ApiEndpoint.Unreachable = true
	}

	for _, n := range description.Items {
		if strings.ToLower(n.Kind) == "node" {
			// By default assume node is ready.
			//
			// The not-ready status needs to be explicitly
			// read from the output of kubectl.
			isReady := true
			transitionTime := (*metav1.Time)(nil)
			for _, cond := range n.Status.Conditions {
				if cond.Type == corev1.NodeReady {
					transitionTime = cond.LastTransitionTime.DeepCopy()
					if cond.Status != corev1.ConditionTrue {
						logger.
							Warn().
							Msgf("Kubernetes node %q is unhealthy with status: %q",
								n.Metadata.Name,
								cond.Status,
							)

						isReady = false
					}
				}
			}

			result.Cluster.Nodes[n.Metadata.Name] = &NodeDescription{
				K8sName:            n.Metadata.Name,
				Ready:              isReady,
				LastTransitionTime: transitionTime,
			}
		}
	}

	if len(result.Cluster.Nodes) > 0 {
		// Match the nodes with their public IPv4.
		for _, np := range state.K8S.ClusterInfo.NodePools {
			for _, n := range np.Nodes {
				// kubernetes names have stripped cluster prefix.
				strippedName := strings.TrimPrefix(n.Name, fmt.Sprintf("%s-", state.K8S.ClusterInfo.Id()))

				if v, ok := result.Cluster.Nodes[strippedName]; ok {
					v.IsStatic = np.GetStaticNodePool() != nil
					v.NodePool = np.Name
					v.PublicIPv4 = n.Public
					v.IsControl = np.IsControl
				}
			}
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

	username, public, key, sshPort := nodepools.RandomNodePublicEndpoint(nps)
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
