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
	"time"
)

// ErrCloudflareAPIForbidden is returned when the response to the endpoint of cloudflare returns code 403,
// which means that the endpoint cannot be reached with the current account-id/token pair.
var ErrCloudflareAPIForbidden = errors.New("token/account-id pair with the cloudflare provider does not have acces for the necessary API")

// Id returns the ID of the cluster.
func (c *ClusterInfo) Id() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s-%s", c.Name, c.Hash)
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
	case *Provider_Oci:
		return p.Oci.PrivateKey
	case *Provider_Aws:
		return p.Aws.SecretKey
	case *Provider_Azure:
		return p.Azure.ClientSecret
	case *Provider_Cloudflare:
		return p.Cloudflare.Token
	case *Provider_Openstack:
		return p.Openstack.ApplicationCredentialSecret
	case *Provider_Exoscale:
		return p.Exoscale.ApiSecret
	case *Provider_Cloudrift:
		return p.Cloudrift.Token
	case *Provider_Verda:
		return p.Verda.ClientSecret
	default:
		panic(fmt.Sprintf("unexpected type %T", pr.ProviderType))
	}
}

func (pr *Provider) CopyCredentials(other *Provider) {
	if pr == nil {
		return
	}

	if other == nil {
		return
	}

	switch p := pr.ProviderType.(type) {
	case *Provider_Aws:
		o, ok := other.ProviderType.(*Provider_Aws)
		if !ok {
			return
		}

		p.Aws.AccessKey = o.Aws.AccessKey
		p.Aws.SecretKey = o.Aws.SecretKey
	case *Provider_Azure:
		o, ok := other.ProviderType.(*Provider_Azure)
		if !ok {
			return
		}

		p.Azure.ClientSecret = o.Azure.ClientSecret
	case *Provider_Cloudflare:
		o, ok := other.ProviderType.(*Provider_Cloudflare)
		if !ok {
			return
		}

		p.Cloudflare.Token = o.Cloudflare.Token
	case *Provider_Cloudrift:
		o, ok := other.ProviderType.(*Provider_Cloudrift)
		if !ok {
			return
		}

		p.Cloudrift.Token = o.Cloudrift.Token
	case *Provider_Exoscale:
		o, ok := other.ProviderType.(*Provider_Exoscale)
		if !ok {
			return
		}

		p.Exoscale.ApiSecret = o.Exoscale.ApiSecret
		p.Exoscale.ApiKey = o.Exoscale.ApiKey
	case *Provider_Gcp:
		o, ok := other.ProviderType.(*Provider_Gcp)
		if !ok {
			return
		}

		p.Gcp.Key = o.Gcp.Key
	case *Provider_Hetzner:
		o, ok := other.ProviderType.(*Provider_Hetzner)
		if !ok {
			return
		}

		p.Hetzner.Token = o.Hetzner.Token
	case *Provider_Oci:
		o, ok := other.ProviderType.(*Provider_Oci)
		if !ok {
			return
		}

		p.Oci.KeyFingerprint = o.Oci.KeyFingerprint
		p.Oci.PrivateKey = o.Oci.PrivateKey
	case *Provider_Openstack:
		o, ok := other.ProviderType.(*Provider_Openstack)
		if !ok {
			return
		}

		p.Openstack.ApplicationCredentialID = o.Openstack.ApplicationCredentialID
		p.Openstack.ApplicationCredentialSecret = o.Openstack.ApplicationCredentialSecret
	case *Provider_Verda:
		o, ok := other.ProviderType.(*Provider_Verda)
		if !ok {
			return
		}

		p.Verda.ClientId = o.Verda.ClientId
		p.Verda.ClientSecret = o.Verda.ClientSecret
		p.Verda.BaseUrl = o.Verda.BaseUrl
	default:
		// do nothing.
	}
}

// Checks whether these two providers have the same credentials.
func (pr *Provider) CredentialsEqual(other *Provider) (equal bool) {
	if pr == nil {
		return
	}

	if other == nil {
		return
	}

	switch p := pr.ProviderType.(type) {
	case *Provider_Aws:
		o, ok := other.ProviderType.(*Provider_Aws)
		if !ok {
			return
		}

		accessKey := p.Aws.AccessKey == o.Aws.AccessKey
		secretKey := p.Aws.SecretKey == o.Aws.SecretKey

		equal = accessKey && secretKey
	case *Provider_Azure:
		o, ok := other.ProviderType.(*Provider_Azure)
		if !ok {
			return
		}

		equal = p.Azure.ClientSecret == o.Azure.ClientSecret
	case *Provider_Cloudflare:
		o, ok := other.ProviderType.(*Provider_Cloudflare)
		if !ok {
			return
		}

		equal = p.Cloudflare.Token == o.Cloudflare.Token
	case *Provider_Cloudrift:
		o, ok := other.ProviderType.(*Provider_Cloudrift)
		if !ok {
			return
		}

		equal = p.Cloudrift.Token == o.Cloudrift.Token
	case *Provider_Exoscale:
		o, ok := other.ProviderType.(*Provider_Exoscale)
		if !ok {
			return
		}

		apiSecret := p.Exoscale.ApiSecret == o.Exoscale.ApiSecret
		apiKey := p.Exoscale.ApiKey == o.Exoscale.ApiKey

		equal = apiSecret && apiKey
	case *Provider_Gcp:
		o, ok := other.ProviderType.(*Provider_Gcp)
		if !ok {
			return
		}

		equal = p.Gcp.Key == o.Gcp.Key
	case *Provider_Hetzner:
		o, ok := other.ProviderType.(*Provider_Hetzner)
		if !ok {
			return
		}

		equal = p.Hetzner.Token == o.Hetzner.Token
	case *Provider_Oci:
		o, ok := other.ProviderType.(*Provider_Oci)
		if !ok {
			return
		}

		fingerprint := p.Oci.KeyFingerprint == o.Oci.KeyFingerprint
		key := p.Oci.PrivateKey == o.Oci.PrivateKey

		equal = fingerprint && key
	case *Provider_Openstack:
		o, ok := other.ProviderType.(*Provider_Openstack)
		if !ok {
			return
		}

		id := p.Openstack.ApplicationCredentialID == o.Openstack.ApplicationCredentialID
		secret := p.Openstack.ApplicationCredentialSecret == o.Openstack.ApplicationCredentialSecret

		equal = id && secret
	case *Provider_Verda:
		o, ok := other.ProviderType.(*Provider_Verda)
		if !ok {
			return
		}

		clientID := p.Verda.ClientId == o.Verda.ClientId
		clientSecret := p.Verda.ClientSecret == o.Verda.ClientSecret
		baseURL := p.Verda.GetBaseUrl() == o.Verda.GetBaseUrl()

		equal = clientID && clientSecret && baseURL
	default:
		// do nothing.
	}

	return
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
				return false, ErrCloudflareAPIForbidden
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
	return false, nil
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
