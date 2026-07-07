package manifest

import (
	"fmt"
	"math"
	"strings"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	k8sV1 "k8s.io/api/core/v1"
)

var (
	// defaultDiskSize defines size of the disk if not specified in manifest.
	// 50GB is the smallest disk size commonly supported by all the cloud providers
	// supported by Claudie.
	defaultDiskSize int32 = 50
)

// GetProvider will search for a Provider config by matching name from providerSpec
// returns *spec.Provider,nil if matching Provider config found otherwise returns nil,error
// This function should only be called after the default templates were set by the operator.
func (ds *Manifest) GetProvider(providerSpecName string) (*spec.Provider, error) {
	for _, gcpConf := range ds.Providers.GCP {
		if gcpConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(gcpConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", gcpConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}

			return &spec.Provider{
				SpecName: gcpConf.Name,
				ProviderType: &spec.Provider_Gcp{
					Gcp: &spec.GCPProvider{
						Key:     gcpConf.Credentials,
						Project: gcpConf.GCPProject,
					},
				},
				CloudProviderName: "gcp",
				Templates:         t,
				//omit rest of the spec.Provider variables
			}, nil
		}
	}

	for _, hetznerConf := range ds.Providers.Hetzner {
		if hetznerConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(hetznerConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", hetznerConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: hetznerConf.Name,
				ProviderType: &spec.Provider_Hetzner{
					Hetzner: &spec.HetznerProvider{
						Token: hetznerConf.Credentials,
					},
				},
				CloudProviderName: "hetzner",
				Templates:         t,
				//omit rest of the spec.Provider variables
			}, nil
		}
	}

	for _, ociConf := range ds.Providers.OCI {
		if ociConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(ociConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", ociConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: ociConf.Name,
				ProviderType: &spec.Provider_Oci{
					Oci: &spec.OCIProvider{
						UserOCID:        ociConf.UserOCID,
						TenancyOCID:     ociConf.TenancyOCID,
						KeyFingerprint:  ociConf.KeyFingerprint,
						CompartmentOCID: ociConf.CompartmentID,
						PrivateKey:      ociConf.PrivateKey,
					},
				},
				CloudProviderName: "oci",
				Templates:         t,
				//omit rest of the spec.Provider variables
			}, nil
		}
	}

	for _, azureConf := range ds.Providers.Azure {
		if azureConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(azureConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", azureConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName:          azureConf.Name,
				CloudProviderName: "azure",
				ProviderType: &spec.Provider_Azure{
					Azure: &spec.AzureProvider{
						SubscriptionID: azureConf.SubscriptionId,
						TenantID:       azureConf.TenantId,
						ClientID:       azureConf.ClientId,
						ClientSecret:   azureConf.ClientSecret,
					},
				},
				Templates: t,
				//omit rest of the pb.Provider variables
			}, nil
		}
	}

	for _, awsConf := range ds.Providers.AWS {
		if awsConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(awsConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", awsConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: awsConf.Name,
				ProviderType: &spec.Provider_Aws{
					Aws: &spec.AWSProvider{
						SecretKey: awsConf.SecretKey,
						AccessKey: awsConf.AccessKey,
					},
				},
				CloudProviderName: "aws",
				Templates:         t,
			}, nil
		}
	}

	for _, cloudflareConf := range ds.Providers.Cloudflare {
		if cloudflareConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(cloudflareConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", cloudflareConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Cloudflare{
					Cloudflare: &spec.CloudflareProvider{
						Token:     cloudflareConf.ApiToken,
						AccountID: cloudflareConf.AccountID,
					},
				},
				CloudProviderName: "cloudflare",
				Templates:         t,
			}, nil
		}
	}

	for _, os := range ds.Providers.Openstack {
		if os.Name == providerSpecName {
			t, err := convertToGrpcTemplates(os.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", os.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Openstack{
					Openstack: &spec.OpenstackProvider{
						AuthURL:                     os.AuthURL,
						DomainID:                    os.DomainId,
						ProjectID:                   os.ProjectId,
						ApplicationCredentialID:     os.ApplicationCredentialId,
						ApplicationCredentialSecret: os.ApplicationCredentialSecret,
					},
				},
				CloudProviderName: "openstack",
				Templates:         t,
			}, nil
		}
	}

	for _, exoConf := range ds.Providers.Exoscale {
		if exoConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(exoConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", exoConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Exoscale{
					Exoscale: &spec.ExoscaleProvider{
						ApiKey:    exoConf.ApiKey,
						ApiSecret: exoConf.ApiSecret,
					},
				},
				CloudProviderName: "exoscale",
				Templates:         t,
			}, nil
		}
	}

	for _, crConf := range ds.Providers.CloudRift {
		if crConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(crConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", crConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			cr := &spec.CloudRiftProvider{
				Token: crConf.Token,
			}
			if crConf.TeamId != "" {
				cr.TeamId = &crConf.TeamId
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Cloudrift{
					Cloudrift: cr,
				},
				CloudProviderName: "cloudrift",
				Templates:         t,
			}, nil
		}
	}

	for _, vConf := range ds.Providers.Verda {
		if vConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(vConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", vConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			v := &spec.VerdaProvider{
				ClientId:     vConf.ClientId,
				ClientSecret: vConf.ClientSecret,
			}
			if vConf.BaseUrl != "" {
				v.BaseUrl = &vConf.BaseUrl
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Verda{
					Verda: v,
				},
				CloudProviderName: "verda",
				Templates:         t,
			}, nil
		}
	}

	for _, oConf := range ds.Providers.OVH {
		if oConf.Name == providerSpecName {
			t, err := convertToGrpcTemplates(oConf.Templates)
			if err != nil {
				return nil, fmt.Errorf("failed to convert template for provider %q: %w", oConf.Name, err)
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			o := &spec.OVHProvider{
				ClientId:     oConf.ClientId,
				ClientSecret: oConf.ClientSecret,
				ServiceName:  oConf.ServiceName,
			}
			if oConf.Endpoint != "" {
				o.Endpoint = &oConf.Endpoint
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Ovh{
					Ovh: o,
				},
				CloudProviderName: "ovh",
				Templates:         t,
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to find provider with name: %s", providerSpecName)
}

// IsKubernetesClusterPresent checks in the manifests if a cluster
// was defined with the specified name.
func (m *Manifest) IsKubernetesClusterPresent(name string) bool {
	for _, c := range m.Kubernetes.Clusters {
		if c.Name == name {
			return true
		}
	}
	return false
}

// FindDynamicNodePool will search for the nodepool in manifest.DynamicNodePool based on the nodepool name
// returns *manifest.DynamicNodePool if found, nil otherwise
func (ds *Manifest) FindDynamicNodePool(nodePoolName string) *DynamicNodePool {
	for _, nodePool := range ds.NodePools.Dynamic {
		if nodePool.Name == nodePoolName {
			return &nodePool
		}
	}
	return nil
}

// FindStaticNodePool will search for the nodepool in manifest.StaticNodePool based on the nodepool name
// returns *manifest.StaticNodePool if found, nil otherwise
func (ds *Manifest) FindStaticNodePool(nodePoolName string) *StaticNodePool {
	for _, nodePool := range ds.NodePools.Static {
		if nodePool.Name == nodePoolName {
			return &nodePool
		}
	}
	return nil
}

// CreateNodepools will create a pb.Nodepool structs based on the manifest specification
// returns error if nodepool/provider not defined, nil otherwise
func (ds *Manifest) CreateNodepools(pools []string, isControl bool) ([]*spec.NodePool, error) {
	var nodePools []*spec.NodePool
	for _, nodePoolName := range pools {
		// Check if the nodepool is part of the cluster
		if nodePool := ds.FindDynamicNodePool(nodePoolName); nodePool != nil {
			provider, err := ds.GetProvider(nodePool.ProviderSpec.Name)
			if err != nil {
				return nil, err
			}

			// Check if autoscaler is defined
			var autoscalerConf *spec.AutoscalerConf
			count := nodePool.Count
			if nodePool.AutoscalerConfig.isDefined() {
				autoscalerConf = &spec.AutoscalerConf{
					Min: nodePool.AutoscalerConfig.Min,
					Max: nodePool.AutoscalerConfig.Max,

					// TargetSize is the desired capacity of
					// the autoscaled nodepool and the `count`
					// of the dynamic nodepool should slowly
					// approach this value as the nodepool is
					// reconciled with time.
					TargetSize: nodePool.AutoscalerConfig.Min,
				}

				// For fresh autoscaled nodepools keep the count
				// equal to the `TargetSize`. The target Size is
				// not managed by the InputManifest and is actually
				// managed by the 'cluster-autoscaler' service that
				// is external to the Manager, thus the `TargetSize`
				// is externally managed and the correct `TargetSize`
				// will be resolved at a later stage when merging with
				// existing state is done, if it already exists.
				//
				// See existing_state.go:[transferDynamicNodePool]
				count = autoscalerConf.TargetSize
			}

			// Set default disk size if not defined. (Value only used in compute nodepools)
			if nodePool.StorageDiskSize == nil {
				nodePool.StorageDiskSize = &defaultDiskSize
			}

			var machineSpec *spec.MachineSpec
			if nodePool.MachineSpec != nil {
				// Use NvidiaGpuCount as primary, fall back to deprecated NvidiaGpu for backward compatibility
				gpuCount := int32(nodePool.MachineSpec.NvidiaGpuCount)
				if gpuCount == 0 && nodePool.MachineSpec.NvidiaGpu > 0 {
					gpuCount = int32(nodePool.MachineSpec.NvidiaGpu)
				}
				machineSpec = &spec.MachineSpec{
					CpuCount:       int32(nodePool.MachineSpec.CpuCount),
					Memory:         int32(nodePool.MachineSpec.Memory),
					NvidiaGpuCount: gpuCount,
					NvidiaGpuType:  nodePool.MachineSpec.NvidiaGpuType,
				}
			}

			nodePools = append(nodePools, &spec.NodePool{
				Name:        nodePool.Name,
				IsControl:   isControl,
				Labels:      nodePool.Labels,
				Annotations: nodePool.Annotations,
				Taints:      getTaints(nodePool.Taints),
				// The nodes are left empty, as the desired state
				// in the manifest does not specify each of the nodes
				// individually, just the count of the nodes that the
				// nodepools should have. The nodes themselves will
				// be resolved at a later step in the build pipeline.
				Nodes: nil,
				Type: &spec.NodePool_DynamicNodePool{
					DynamicNodePool: &spec.DynamicNodePool{
						Region:              nodePool.ProviderSpec.Region,
						Zone:                nodePool.ProviderSpec.Zone,
						ServerType:          nodePool.ServerType,
						Image:               nodePool.Image,
						ExternalNetworkName: nodePool.ProviderSpec.ExternalNetworkName,
						StorageDiskSize:     *nodePool.StorageDiskSize,
						Count:               count,
						Provider:            provider,
						AutoscalerConfig:    autoscalerConf,
						MachineSpec:         machineSpec,
						Spot:                nodePool.Spot,
					},
				},
			})
		} else if nodePool := ds.FindStaticNodePool(nodePoolName); nodePool != nil {
			nodes := staticNodes(nodePool, isControl)
			taints := getTaints(nodePool.Taints)
			keys := getNodeKeys(nodePool)

			nodePools = append(nodePools, &spec.NodePool{
				Name: nodePool.Name,
				// Contrary to the dynamic nodepools, The nodes
				// for the static nodepools are explicitly defined
				// in the manifest itself, thus they already are stored
				// in this step of the build pipeline.
				Nodes:       nodes,
				IsControl:   isControl,
				Labels:      nodePool.Labels,
				Annotations: nodePool.Annotations,
				Taints:      taints,
				SshPort:     resolveSSHPort(nodePool.SshPort),
				Type: &spec.NodePool_StaticNodePool{
					StaticNodePool: &spec.StaticNodePool{
						NodeKeys: keys,
					},
				},
			})
		} else {
			return nil, fmt.Errorf("nodepool %s not defined", nodePoolName)
		}
	}
	return nodePools, nil
}

func FetchCommitHash(tmpl *spec.TemplateRepository) error {
	isGitHash := func(s string) bool {
		// Git uses either sha1 or sha256 hashes thus either 40 byte or 64 byte lengths.
		if len(s) != 40 && len(s) != 64 { // SHA-1 or SHA-256 object id
			return false
		}
		for _, c := range s {
			switch {
			case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
			default:
				return false
			}
		}
		return true
	}

	if isGitHash(tmpl.Commit) {
		tmpl.CommitHash = tmpl.Commit
		return nil
	}

	var repository string
	switch tmpl.Endpoint.Protocol {
	case spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS:
		repository = tmpl.HttpsUrl()
	default:
		return fmt.Errorf("protocol %q is unsupported for retrieving commit hashes", tmpl.Endpoint.Protocol)
	}

	cfg := config.RemoteConfig{
		Name: "origin",
		URLs: []string{repository},
	}
	opts := git.ListOptions{
		Timeout:       40,
		PeelingOption: git.AppendPeeled,
	}

	if tmpl.Auth != nil {
		auth := http.BasicAuth{
			Username: "x-access-token",
			Password: tmpl.Auth.Token,
		}
		if tmpl.Auth.Username != nil {
			auth.Username = *tmpl.Auth.Username
		}
		opts.Auth = &auth
	}

	r := git.NewRemote(memory.NewStorage(), &cfg)
	rfs, err := r.List(&opts)
	if err != nil {
		return fmt.Errorf("failed to list remote repository %q: %w", repository, err)
	}

	var branchCommit, tagCommit, tagPeeledCommit string
	for _, r := range rfs {
		switch {
		case r.Name().IsBranch() && r.Name().Short() == tmpl.Commit:
			branchCommit = r.Hash().String()
		case r.Name().IsTag() && r.Name().Short() == tmpl.Commit:
			tagCommit = r.Hash().String()
		case r.Name().IsTag() && r.Name().Short() == tmpl.Commit+"^{}": // annotated tag
			tagPeeledCommit = r.Hash().String()
		}
	}

	switch {
	case tagPeeledCommit != "":
		tmpl.CommitHash = tagPeeledCommit
	case tagCommit != "":
		tmpl.CommitHash = tagCommit
	case branchCommit != "":
		tmpl.CommitHash = branchCommit
	default:
		return fmt.Errorf("couldn't find the requested commit %q for the template repository %q", tmpl.Commit, repository)
	}

	return nil
}

// staticNodes returns slice of static nodes with initialised name.
func staticNodes(np *StaticNodePool, isControl bool) []*spec.Node {
	if len(np.Nodes) > math.MaxUint8 {
		panic(fmt.Sprintf("static nodepool %q defined more than 255 nodes, which is the claudie internal maximum", np.Name))
	}

	nodes := make([]*spec.Node, 0, len(np.Nodes))
	nodeType := spec.NodeType_worker
	if isControl {
		nodeType = spec.NodeType_master
	}

	for i, node := range np.Nodes {
		nodes = append(nodes, &spec.Node{
			// Name only matters on the first run of the static nodepool,
			// on subsequent runs, if there are previously build nodes
			// with the same public IP we will transfer that existing name.
			// see existing_state.go:[transferStaticNodePool]
			// Further, the name is not used for "determining" if the
			// node is used in any previous or any other state, for that
			// the Public endpoint should be used which is an Unique Identifier
			// of the node. If this changes in the future, relevant code may
			// need to be adjusted.
			Name:     fmt.Sprintf("%s-%02x", np.Name, uint8(i+1)),
			Public:   node.Endpoint,
			NodeType: nodeType,
			Status:   spec.NodeStatus_Preparing,
			Username: node.Username,
		})
	}

	return nodes
}

func resolveSSHPort(port *int32) int32 {
	if port == nil {
		return nodepools.DefaultSSHPort
	}
	return *port
}

// getNodeKeys returns map of keys for static nodes in map[endpoint]key.
func getNodeKeys(nodepool *StaticNodePool) map[string]string {
	m := make(map[string]string)
	for _, n := range nodepool.Nodes {
		m[n.Endpoint] = n.Key
	}
	return m
}

func getTaints(taints []k8sV1.Taint) []*spec.Taint {
	arr := make([]*spec.Taint, 0, len(taints))
	for _, t := range taints {
		arr = append(arr, &spec.Taint{Key: t.Key, Value: t.Value, Effect: string(t.Effect)})
	}
	return arr
}

// nodePoolDefined returns true if node pool is defined in manifest, false otherwise.
func (ds *Manifest) nodePoolDefined(pool string) (defined bool, static bool) {
	for _, nodePool := range ds.NodePools.Static {
		if nodePool.Name == pool {
			return true, true
		}
	}
	for _, nodePool := range ds.NodePools.Dynamic {
		if nodePool.Name == pool {
			return true, false
		}
	}

	return
}

func (ds *Manifest) GetProviderType(provider string) (string, error) {
	var t string

	ds.ForEachProvider(func(name, typ string) bool {
		if name == provider {
			t = typ
			return false
		}
		return true
	})

	if t == "" {
		return "", fmt.Errorf("failed to find provider %s", provider)
	}

	return t, nil
}

func (ds *Manifest) ForEachProvider(do func(name, typ string) bool) {
	for _, c := range ds.Providers.GCP {
		if !do(c.Name, "gcp") {
			return
		}
	}
	for _, c := range ds.Providers.Hetzner {
		if !do(c.Name, "hetzner") {
			return
		}
	}
	for _, c := range ds.Providers.OCI {
		if !do(c.Name, "oci") {
			return
		}
	}
	for _, c := range ds.Providers.AWS {
		if !do(c.Name, "aws") {
			return
		}
	}
	for _, c := range ds.Providers.Azure {
		if !do(c.Name, "azure") {
			return
		}
	}
	for _, c := range ds.Providers.Cloudflare {
		if !do(c.Name, "cloudflare") {
			return
		}
	}
	for _, c := range ds.Providers.Openstack {
		if !do(c.Name, "openstack") {
			return
		}
	}
	for _, c := range ds.Providers.Exoscale {
		if !do(c.Name, "exoscale") {
			return
		}
	}
	for _, c := range ds.Providers.CloudRift {
		if !do(c.Name, "cloudrift") {
			return
		}
	}
	for _, c := range ds.Providers.Verda {
		if !do(c.Name, "verda") {
			return
		}
	}
	for _, c := range ds.Providers.OVH {
		if !do(c.Name, "ovh") {
			return
		}
	}
}

func convertToGrpcTemplates(t *TemplateRepository) (*spec.TemplateRepository, error) {
	var protocol spec.TemplateRepository_Endpoint_Protocol
	switch strings.ToLower(t.Endpoint.Protocol) {
	case "https":
		protocol = spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS
	default:
		return nil, fmt.Errorf("unsupported protocol %v", t.Endpoint.Protocol)
	}

	out := &spec.TemplateRepository{
		Endpoint: &spec.TemplateRepository_Endpoint{
			Url:      t.Endpoint.URL,
			Protocol: protocol,
		},
		Auth:   nil,
		Commit: t.Commit,
		Paths: &spec.TemplateRepository_TemplatePaths{
			Terraformer:  t.Paths.Terraformer,
			Playbooks:    t.Paths.Playbooks,
			ConfigLb:     t.Paths.ConfigLb,
			ConfigK8S:    t.Paths.ConfigK8s,
			ManifestsK8S: t.Paths.ManifestsK8s,
		},
	}

	if t.Auth != nil {
		out.Auth = &spec.TemplateRepository_Auth{Token: t.Auth.Token}
		if t.Auth.Username != "" {
			out.Auth.Username = new(t.Auth.Username)
		}
	}

	return out, nil
}
