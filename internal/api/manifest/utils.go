package manifest

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
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
			t := &spec.TemplateRepository{
				Repository: gcpConf.Templates.Repository,
				Tag:        gcpConf.Templates.Tag,
				Path:       gcpConf.Templates.Path,
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
			t := &spec.TemplateRepository{
				Repository: hetznerConf.Templates.Repository,
				Tag:        hetznerConf.Templates.Tag,
				Path:       hetznerConf.Templates.Path,
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
			t := &spec.TemplateRepository{
				Repository: ociConf.Templates.Repository,
				Tag:        ociConf.Templates.Tag,
				Path:       ociConf.Templates.Path,
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
			t := &spec.TemplateRepository{
				Repository: azureConf.Templates.Repository,
				Tag:        azureConf.Templates.Tag,
				Path:       azureConf.Templates.Path,
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
			t := &spec.TemplateRepository{
				Repository: awsConf.Templates.Repository,
				Tag:        awsConf.Templates.Tag,
				Path:       awsConf.Templates.Path,
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
			t := &spec.TemplateRepository{
				Repository: cloudflareConf.Templates.Repository,
				Tag:        cloudflareConf.Templates.Tag,
				Path:       cloudflareConf.Templates.Path,
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

	for _, hetznerDNSConfig := range ds.Providers.HetznerDNS {
		if hetznerDNSConfig.Name == providerSpecName {
			t := &spec.TemplateRepository{
				Repository: hetznerDNSConfig.Templates.Repository,
				Tag:        hetznerDNSConfig.Templates.Tag,
				Path:       hetznerDNSConfig.Templates.Path,
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Hetznerdns{
					Hetznerdns: &spec.HetznerDNSProvider{
						Token: hetznerDNSConfig.ApiToken,
					},
				},
				CloudProviderName: "hetznerdns",
				Templates:         t,
			}, nil
		}
	}

	for _, gc := range ds.Providers.GenesisCloud {
		if gc.Name == providerSpecName {
			t := &spec.TemplateRepository{
				Repository: gc.Templates.Repository,
				Tag:        gc.Templates.Tag,
				Path:       gc.Templates.Path,
			}
			if err := FetchCommitHash(t); err != nil {
				return nil, err
			}
			return &spec.Provider{
				SpecName: providerSpecName,
				ProviderType: &spec.Provider_Genesiscloud{
					Genesiscloud: &spec.GenesisCloudProvider{
						Token: gc.ApiToken,
					},
				},
				CloudProviderName: "genesiscloud",
				Templates:         t,
			}, nil
		}
	}
	for _, os := range ds.Providers.Openstack {
		if os.Name == providerSpecName {
			t := &spec.TemplateRepository{
				Repository: os.Templates.Repository,
				Tag:        os.Templates.Tag,
				Path:       os.Templates.Path,
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
				machineSpec = &spec.MachineSpec{
					CpuCount:  int32(nodePool.MachineSpec.CpuCount),
					Memory:    int32(nodePool.MachineSpec.Memory),
					NvidiaGpu: int32(nodePool.MachineSpec.NvidiaGpu),
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
	r := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{tmpl.Repository},
	})

	rfs, err := r.List(&git.ListOptions{
		Timeout: 60,
	})
	if err != nil {
		return fmt.Errorf("failed to list remote repository %q: %w", tmpl.Repository, err)
	}

	if tmpl.Tag != nil {
		rfs = slices.DeleteFunc(rfs, func(reference *plumbing.Reference) bool {
			//nolint
			return !(reference.Name().IsTag() && reference.Name().Short() == *tmpl.Tag)
		})
	} else {
		i := slices.IndexFunc(rfs, func(r *plumbing.Reference) bool { return r.Name().Short() == "HEAD" })
		if i < 0 {
			return errors.New("couldn't find commit hash for HEAD of the template repository")
		}
		t := rfs[i].Target()
		rfs = slices.DeleteFunc(rfs, func(r *plumbing.Reference) bool { return r.Name() != t })
	}

	if len(rfs) != 1 {
		target := "HEAD"
		if tmpl.Tag != nil {
			target = *tmpl.Tag
		}
		return fmt.Errorf("couldn't find the requested target %q, for the template repository %q", target, tmpl.Repository)
	}

	tmpl.CommitHash = rfs[0].Hash().String()
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

	ds.ForEachProvider(func(name, typ string, _ **TemplateRepository) bool {
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

func (ds *Manifest) ForEachProvider(do func(name, typ string, tmpls **TemplateRepository) bool) {
	for i, c := range ds.Providers.GCP {
		if !do(c.Name, "gcp", &ds.Providers.GCP[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.Hetzner {
		if !do(c.Name, "hetzner", &ds.Providers.Hetzner[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.OCI {
		if !do(c.Name, "oci", &ds.Providers.OCI[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.AWS {
		if !do(c.Name, "aws", &ds.Providers.AWS[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.Azure {
		if !do(c.Name, "azure", &ds.Providers.Azure[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.GenesisCloud {
		if !do(c.Name, "genesiscloud", &ds.Providers.GenesisCloud[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.Cloudflare {
		if !do(c.Name, "cloudflare", &ds.Providers.Cloudflare[i].Templates) {
			return
		}
	}

	for i, c := range ds.Providers.HetznerDNS {
		if !do(c.Name, "hetznerdns", &ds.Providers.HetznerDNS[i].Templates) {
			return
		}
	}
	for i, c := range ds.Providers.Openstack {
		if !do(c.Name, "openstack", &ds.Providers.Openstack[i].Templates) {
			return
		}
	}
}
