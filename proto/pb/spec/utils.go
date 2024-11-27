package spec

import "fmt"

// Id returns the ID of the cluster.
func (c *ClusterInfo) Id() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s-%s", c.Name, c.Hash)
}

// DynamicNodePools returns slice of dynamic node pools.
func (c *ClusterInfo) DynamicNodePools() []*DynamicNodePool {
	if c == nil {
		return nil
	}

	nps := make([]*DynamicNodePool, 0, len(c.NodePools))
	for _, np := range c.NodePools {
		if n := np.GetDynamicNodePool(); n != nil {
			nps = append(nps, n)
		}
	}

	return nps
}

// AnyAutoscaledNodePools returns true, if cluster has at least one nodepool with autoscaler config.
func (c *K8Scluster) AnyAutoscaledNodePools() bool {
	if c == nil {
		return false
	}

	for _, np := range c.ClusterInfo.NodePools {
		if n := np.GetDynamicNodePool(); n != nil {
			if n.AutoscalerConfig != nil {
				return true
			}
		}
	}

	return false
}

func (c *K8Scluster) NodeCount() int {
	var out int

	if c == nil {
		return out
	}

	for _, np := range c.ClusterInfo.NodePools {
		switch i := np.Type.(type) {
		case *NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *NodePool_StaticNodePool:
			out += len(i.StaticNodePool.NodeKeys)
		}
	}

	return out
}

func (c *LBcluster) NodeCount() int {
	var out int

	if c == nil {
		return out
	}

	for _, np := range c.ClusterInfo.NodePools {
		switch i := np.Type.(type) {
		case *NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *NodePool_StaticNodePool:
			// Lbs are only dynamic.
		}
	}

	return out
}

func (c *LBcluster) HasApiRole() bool {
	if c == nil {
		return false
	}

	for _, role := range c.Roles {
		if role.RoleType == RoleType_ApiServer {
			return true
		}
	}

	return false
}

// EndpointNode searches for a node with type ApiEndpoint.
func (n *NodePool) EndpointNode() *Node {
	if n == nil {
		return nil
	}

	for _, node := range n.Nodes {
		if node.NodeType == NodeType_apiEndpoint {
			return node
		}
	}

	return nil
}

// Credentials extract the key for the provider to be used within terraform.
func (pr *Provider) Credentials() string {
	if pr == nil {
		return ""
	}

	switch p := pr.ProviderType.(type) {
	case *Provider_Gcp:
		return p.Gcp.Key
	case *Provider_Hetzner:
		return p.Hetzner.Token
	case *Provider_Hetznerdns:
		return p.Hetznerdns.Token
	case *Provider_Oci:
		return p.Oci.PrivateKey
	case *Provider_Aws:
		return p.Aws.SecretKey
	case *Provider_Azure:
		return p.Azure.ClientSecret
	case *Provider_Cloudflare:
		return p.Cloudflare.Token
	case *Provider_Genesiscloud:
		return p.Genesiscloud.Token
	default:
		panic(fmt.Sprintf("unexpected type %T", pr.ProviderType))
	}
}
