package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"time"
)

// ErrCloudflareAPIForbidden is returned when the response to the endpoint of cloudflare returns code 403,
// which means that the endpoint cannot be reached with the current account-id/token pair.
var ErrCloudflareAPIForbidden = errors.New("token/account-id pair with the cloudflare provider does not have acces for the necessary API")

const (
	// ForceExportPort6443OnControlPlane Forces to export the port 6443 on
	// all the control plane nodes in the cluster when the workflow reaches
	// the terraformer stage. This setting applies to the BuildInfrastructure RPC
	// in terraformer.
	ForceExportPort6443OnControlPlane = 1 << iota

	// K8sOnlyRefresh gives a hint to the processing of the task that the task
	// is related only to the k8s cluster infrastructure, thus unrelated infrastructure
	// should be skipped.
	K8sOnlyRefresh
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
		found := slices.Contains(r.TargetPools, o)
		if !found {
			// append missing target pool.
			r.TargetPools = append(r.TargetPools, o)
		}
	}
}

// GetSubscription checks if the Cloudflare account has a Load Balancing subscription.
func (x *CloudflareProvider) GetSubscription() (bool, error) {
	// the number of retries before returning an error on trying to
	// communicate with the cloudflare API.
	const retries = 3

	var subscriptions struct {
		Result []struct {
			ID      string `json:"id"`
			Product struct {
				Name string `json:"name"`
			} `json:"product"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	escapedAccountID := url.PathEscape(x.AccountID)
	urlSubscriptions := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/subscriptions", escapedAccountID)

	var response []byte
	var err error

	// The api seems to fail sometimes, add more checks with a exponential backoff before giving up.
	for i := range retries {
		response, err = getCloudflareAPIResponse(urlSubscriptions, x.Token)
		if err != nil {
			if errors.Is(err, ErrCloudflareAPIForbidden) {
				return false, nil
			}
			time.Sleep((1 << i) * time.Second)
			continue
		}
		break
	}

	if err != nil {
		return false, fmt.Errorf("error while getting cloudflare api response for 'accounts/subscriptions', after %v retries: %w", retries, err)
	}

	if err := json.Unmarshal(response, &subscriptions); err != nil {
		return false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	for _, subscription := range subscriptions.Result {
		if subscription.Product.Name == "prod_load_balancing" && subscriptions.Success {
			return true, nil
		}
	}
	return false, fmt.Errorf("subscription for Load Balancing not found")
}

func getCloudflareAPIResponse(url string, apiToken string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrCloudflareAPIForbidden
	}

	// nolint
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil, fmt.Errorf("response with status code %v: %v", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}
