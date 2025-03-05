package spec

import (
	"fmt"
	"net/url"
	"path/filepath"
)

const (
	// ForceExportPort6443OnControlPlane Forces to export the port 6443 on
	// all the control plane nodes in the cluster when the workflow reaches
	// the terraformer stage. This setting applies to the BuildInfrastructure RPC
	// in terraformer.
	ForceExportPort6443OnControlPlane = 1 << iota
)

func OptionIsSet(options uint64, option uint64) bool { return options&option != 0 }

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

// HasApiRole checks whether the LB has a role with port 6443.
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

// IsApiEndpoint  checks whether the LB is selected as the API endpoint.
func (c *LBcluster) IsApiEndpoint() bool {
	if c == nil {
		return false
	}
	return c.HasApiRole() && c.UsedApiEndpoint
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

// MustExtractTargetPath returns the target path of the external template repository.
// If the URL of the repository is invalid this functions panics.
// The target path is the path where the templates should be downloaded on the local
// filesystem.
func (r *TemplateRepository) MustExtractTargetPath() string {
	if r == nil {
		return ""
	}

	u, err := url.Parse(r.Repository)
	if err != nil {
		panic(err)
	}

	return filepath.Join(
		u.Hostname(),
		u.Path,
		r.CommitHash,
		r.Path,
	)
}

func (n *NodePool) Zone() string {
	var sn string

	switch {
	case n.GetDynamicNodePool() != nil:
		sn = n.GetDynamicNodePool().Provider.SpecName
	case n.GetStaticNodePool() != nil:
		sn = StaticNodepoolInfo_STATIC_PROVIDER.String()
	default:
		panic("unsupported nodepool type")
	}

	if sn == "" {
		panic("no zone specified")
	}

	return fmt.Sprintf("%s-zone", sn)
}

// MergeTargetPools takes the target pools from the other role
// and adds them to this role, ignoring duplicates.
func (r *Role) MergeTargetPools(o *Role) {
	for _, o := range o.TargetPools {
		found := false
		for _, c := range r.TargetPools {
			if o == c {
				found = true
				break
			}
		}
		if !found {
			// append missing target pool.
			r.TargetPools = append(r.TargetPools, o)
		}
	}
}
