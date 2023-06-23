package manifest

import (
	"fmt"

	"github.com/berops/claudie/proto/pb"
	k8sV1 "k8s.io/api/core/v1"
)

const (
	// defaultDiskSize defines size of the disk if not specified in manifest.
	// 50GB is the smallest disk size commonly supported by all the cloud providers
	// supported by Claudie.
	defaultDiskSize = 50
)

// GetProvider will search for a Provider config by matching name from providerSpec
// returns *pb.Provider,nil if matching Provider config found otherwise returns nil,error
func (ds *Manifest) GetProvider(providerSpecName string) (*pb.Provider, error) {
	for _, gcpConf := range ds.Providers.GCP {
		if gcpConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          gcpConf.Name,
				Credentials:       gcpConf.Credentials,
				GcpProject:        gcpConf.GCPProject,
				CloudProviderName: "gcp",
				//omit rest of the pb.Provider variables
			}, nil
		}
	}

	for _, hetznerConf := range ds.Providers.Hetzner {
		if hetznerConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          hetznerConf.Name,
				Credentials:       hetznerConf.Credentials,
				CloudProviderName: "hetzner",
				//omit rest of the pb.Provider variables
			}, nil
		}
	}

	for _, ociConf := range ds.Providers.OCI {
		if ociConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:           ociConf.Name,
				Credentials:        ociConf.PrivateKey,
				CloudProviderName:  "oci",
				OciUserOcid:        ociConf.UserOCID,
				OciTenancyOcid:     ociConf.TenancyOCID,
				OciFingerprint:     ociConf.KeyFingerprint,
				OciCompartmentOcid: ociConf.CompartmentID,
				//omit rest of the pb.Provider variables
			}, nil
		}
	}

	for _, azureConf := range ds.Providers.Azure {
		if azureConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:            azureConf.Name,
				CloudProviderName:   "azure",
				Credentials:         azureConf.ClientSecret,
				AzureSubscriptionId: azureConf.SubscriptionId,
				AzureTenantId:       azureConf.TenantId,
				AzureClientId:       azureConf.ClientId,
				//omit rest of the pb.Provider variables
			}, nil
		}
	}

	for _, awsConf := range ds.Providers.AWS {
		if awsConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          awsConf.Name,
				Credentials:       awsConf.SecretKey,
				AwsAccessKey:      awsConf.AccessKey,
				CloudProviderName: "aws",
			}, nil
		}
	}

	for _, cloudflaceConf := range ds.Providers.Cloudflare {
		if cloudflaceConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          providerSpecName,
				Credentials:       cloudflaceConf.ApiToken,
				CloudProviderName: "cloudflare",
			}, nil
		}
	}

	for _, hetznerDNSConfg := range ds.Providers.HetznerDNS {
		if hetznerDNSConfg.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          providerSpecName,
				Credentials:       hetznerDNSConfg.ApiToken,
				CloudProviderName: "hetznerdns",
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
func (ds *Manifest) CreateNodepools(pools []string, isControl bool) ([]*pb.NodePool, error) {
	var nodePools []*pb.NodePool
	for _, nodePoolName := range pools {
		// Check if the nodepool is part of the cluster
		if nodePool := ds.FindDynamicNodePool(nodePoolName); nodePool != nil {
			provider, err := ds.GetProvider(nodePool.ProviderSpec.Name)
			if err != nil {
				return nil, err
			}

			// Check if autoscaler is defined
			var autoscalerConf *pb.AutoscalerConf
			count := nodePool.Count
			if nodePool.AutoscalerConfig.isDefined() {
				autoscalerConf = &pb.AutoscalerConf{}
				autoscalerConf.Min = nodePool.AutoscalerConfig.Min
				autoscalerConf.Max = nodePool.AutoscalerConfig.Max
				count = nodePool.AutoscalerConfig.Min
			}

			// Set default disk size if not defined. (Value only used in compute nodepools)
			if nodePool.StorageDiskSize == 0 {
				nodePool.StorageDiskSize = defaultDiskSize
			}

			nodePools = append(nodePools, &pb.NodePool{
				Name:      nodePool.Name,
				IsControl: isControl,
				Labels:    nodePool.Labels,
				Taints:    getTaints(nodePool.Taints),
				NodePoolType: &pb.NodePool_DynamicNodePool{
					DynamicNodePool: &pb.DynamicNodePool{
						Region:           nodePool.ProviderSpec.Region,
						Zone:             nodePool.ProviderSpec.Zone,
						ServerType:       nodePool.ServerType,
						Image:            nodePool.Image,
						StorageDiskSize:  uint32(nodePool.StorageDiskSize),
						Count:            count,
						Provider:         provider,
						AutoscalerConfig: autoscalerConf,
					},
				},
			})
		} else if nodePool := ds.FindStaticNodePool(nodePoolName); nodePool != nil {
			nodes := getStaticNodes(nodePool)
			nodePools = append(nodePools, &pb.NodePool{
				Name:      nodePool.Name,
				Nodes:     nodes,
				IsControl: isControl,
				Labels:    nodePool.Labels,
				Taints:    getTaints(nodePool.Taints),
				NodePoolType: &pb.NodePool_StaticNodePool{
					StaticNodePool: &pb.StaticNodePool{
						NodeKeys: getNodeKeys(nodePool),
					},
				},
			})
		} else {
			return nil, fmt.Errorf("nodepool %s not defined", nodePoolName)
		}
	}
	return nodePools, nil
}

// getStaticNodes returns slice of static nodes with initialised name.
func getStaticNodes(np *StaticNodePool) []*pb.Node {
	nodes := make([]*pb.Node, 0, len(np.Nodes))
	for i, node := range np.Nodes {
		nodes = append(nodes, &pb.Node{
			Name:   fmt.Sprintf("%s-%d", np.Name, i),
			Public: node.Endpoint,
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

// nodePoolDefined returns true if node pool is defined in manifest, false otherwise.
func (ds *Manifest) nodePoolDefined(pool string) bool {
	for _, nodePool := range ds.NodePools.Static {
		if nodePool.Name == pool {
			return true
		}
	}
	for _, nodePool := range ds.NodePools.Dynamic {
		if nodePool.Name == pool {
			return true
		}
	}
	return false
}

func getTaints(taints []k8sV1.Taint) []*pb.Taint {
	arr := make([]*pb.Taint, 0, len(taints))
	for _, t := range taints {
		arr = append(arr, &pb.Taint{Key: t.Key, Value: t.Value, Effect: string(t.Effect)})
	}
	return arr
}
