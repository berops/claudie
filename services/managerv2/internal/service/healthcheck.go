package service

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

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
	}
}

// HealthCheck performs healthcheck across the passed in [spec.Clusters] state.
func HealthCheck(logger zerolog.Logger, state *spec.ClustersV2) (HealthCheckStatus, error) {
	var result HealthCheckStatus
	result.Cluster.Nodes = make(map[string]struct{})

	logger.Info().Msg("verifying if all nodes are reachable")

	k, lb, err := clusters.PingNodesV2(logger, state)
	if err != nil {
		if !errors.Is(err, clusters.ErrEchoTimeout) {
			logger.Err(err).Msg("failed to determine if any nodes were unreachable")
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

	return result, nil
}
