package managementcluster

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/proto/pb/spec"
)

// Output directory for the short-lived secrets.
const outputDir = "/tmp"

type (
	// IPPair contains pair of public and private IP for a single node.
	IPPair struct {
		// PublicIP describes public IP.
		PublicIP net.IP `json:"public_ip"`

		// PrivateIP describes private IP.
		PrivateIP net.IP `json:"private_ip"`
	}

	// StaticNodeInfo contains metadata info about static node.
	StaticNodeInfo struct {
		// Endpoint is an endpoint for static nodes in the static node pool.
		Endpoint string `json:"endpoint"`

		// PrivateKey is the private SSH key for the node.
		PrivateKey string `json:"node_private_key"`
	}

	// ClusterMetadata contains metadata for the whole cluster. This metadata will be exported as a secret in management cluster.
	ClusterMetadata struct {
		// DynamicNodepools contains metadata for dynamic nodepools.
		DynamicNodepools map[string]DynamicNodepool `json:"dynamic_nodepools"`

		// StaticNodepools contains metadata for static nodepools.
		StaticNodepools map[string]StaticNodepool `json:"static_nodepools"`

		// DynamicLoadBalancerNodePools contain metadata for dynamic lb nodepools.
		DynamicLoadBalancerNodePools map[string]map[string]DynamicNodepool `json:"dynamic_load_balancer_nodepools"`

		// StaticLoadBalancerNodePools contain metadata for static lb nodepools.
		StaticLoadBalancerNodePools map[string]map[string]StaticNodepool `json:"static_load_balancer_nodepools"`
	}

	// DynamicNodepool contains map of node names and their IP pair.
	DynamicNodepool struct {
		// NodeIps maps node-name to public-private ip pairs for dynamic node pools.
		NodeIps map[string]IPPair `json:"node_ips"`

		// PrivateKey is the private SSH key for the dynamic nodes.
		PrivateKey string `json:"nodepool_private_key"`
	}

	// StaticNodepool contains map of node names and their static metadata.
	StaticNodepool struct {
		// NodeIps maps node-name to endpoint-key pairs for static node pools.
		NodeInfo map[string]StaticNodeInfo `json:"node_info"`
	}
)

// StoreClusterMetadata constructs ClusterMetadata for the given K8s and Loadbalancer
// clusters. The output is stored as a secret in the Claudie management cluster.
func StoreClusterMetadata(manifestName string, clusters *spec.Clusters) error {
	// local deployment
	if envs.Namespace == "" {
		return nil
	}

	dp := make(map[string]DynamicNodepool)
	sp := make(map[string]StaticNodepool)

	for _, pool := range clusters.K8S.ClusterInfo.NodePools {
		if np := pool.GetDynamicNodePool(); np != nil {
			dp[pool.Name] = DynamicNodepool{
				NodeIps:    make(map[string]IPPair),
				PrivateKey: np.PrivateKey,
			}
			for _, node := range pool.Nodes {
				dp[pool.Name].NodeIps[node.Name] = IPPair{
					PublicIP:  net.ParseIP(node.Public),
					PrivateIP: net.ParseIP(node.Private),
				}
			}
		} else if np := pool.GetStaticNodePool(); np != nil {
			sp[pool.Name] = StaticNodepool{
				NodeInfo: make(map[string]StaticNodeInfo),
			}
			for _, node := range pool.Nodes {
				sp[pool.Name].NodeInfo[node.Name] = StaticNodeInfo{
					PrivateKey: np.NodeKeys[node.Public],
					Endpoint:   node.Public,
				}
			}
		}
	}

	lbdp := make(map[string]map[string]DynamicNodepool)
	lbst := make(map[string]map[string]StaticNodepool)

	for _, lb := range clusters.LoadBalancers.Clusters {
		lbdp[lb.ClusterInfo.Name] = make(map[string]DynamicNodepool)
		for _, pool := range lb.ClusterInfo.NodePools {
			if np := pool.GetDynamicNodePool(); np != nil {
				lbdp[lb.ClusterInfo.Name][pool.Name] = DynamicNodepool{
					NodeIps:    make(map[string]IPPair),
					PrivateKey: np.PrivateKey,
				}
				for _, node := range pool.Nodes {
					lbdp[lb.ClusterInfo.Name][pool.Name].NodeIps[node.Name] = IPPair{
						PublicIP:  net.ParseIP(node.Public),
						PrivateIP: net.ParseIP(node.Private),
					}
				}
			} else if np := pool.GetStaticNodePool(); np != nil {
				lbst[lb.ClusterInfo.Name][pool.Name] = StaticNodepool{
					NodeInfo: make(map[string]StaticNodeInfo),
				}

				for _, node := range pool.Nodes {
					lbst[lb.ClusterInfo.Name][pool.Name].NodeInfo[node.Name] = StaticNodeInfo{
						PrivateKey: np.NodeKeys[node.Public],
						Endpoint:   node.Public,
					}
				}
			}
		}
	}

	md := ClusterMetadata{
		DynamicNodepools:             dp,
		StaticNodepools:              sp,
		DynamicLoadBalancerNodePools: lbdp,
		StaticLoadBalancerNodePools:  lbst,
	}

	b, err := json.Marshal(md)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster metadata: %w", err)
	}

	var (
		clusterID      = clusters.K8S.ClusterInfo.Id()
		encodedData    = base64.StdEncoding.EncodeToString(b)
		secretData     = map[string]string{"metadata": encodedData}
		secretMetadata = SecretMetadata(clusters.K8S.ClusterInfo, manifestName, MetadataSecret)
		secretYaml     = NewSecretYaml(secretMetadata, secretData)
		clusterDir     = filepath.Join(outputDir, clusterID)
		sec            = NewSecret(clusterDir, secretYaml)
	)

	if err := sec.Apply(envs.Namespace); err != nil {
		return fmt.Errorf("error while creating cluster metadata secret: %w", err)
	}

	return nil
}
