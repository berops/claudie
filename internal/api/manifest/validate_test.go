package manifest

import (
	"math/rand/v2"
	"testing"

	"github.com/berops/claudie/internal/generics"
	"github.com/berops/claudie/internal/spectesting"
	"github.com/stretchr/testify/require"
	k8sV1 "k8s.io/api/core/v1"
)

var (
	testManifest                = &Manifest{NodePools: NodePool{Dynamic: []DynamicNodePool{{Name: "np1"}}}}
	testClusterVersionPass      = &Kubernetes{Clusters: []Cluster{{Name: "cluster1", Network: "10.0.0.0/8", Version: "v1.29.0", Pools: Pool{Control: []string{"np1"}}}}}
	testClusterVersionFailMinor = &Kubernetes{Clusters: []Cluster{{Name: "cluster1", Network: "10.0.0.0/8", Version: "v1.21.0", Pools: Pool{Control: []string{"np1"}}}}}
	testClusterVersionFailMajor = &Kubernetes{Clusters: []Cluster{{Name: "cluster1", Network: "10.0.0.0/8", Version: "v2.22.0", Pools: Pool{Control: []string{"np1"}}}}}

	testNodepoolAutoScalerSuccAC = &DynamicNodePool{Name: "Test", ServerType: "s1", Image: "ubuntu", StorageDiskSize: newIntP(50), AutoscalerConfig: AutoscalerConfig{Min: 1, Max: 3}, ProviderSpec: ProviderSpec{Name: "p1", Region: "a", Zone: "1"}}
	testNodepoolAutoScalerSucc   = &DynamicNodePool{Name: "Test", ServerType: "s1", Image: "ubuntu", StorageDiskSize: newIntP(50), Count: 1, ProviderSpec: ProviderSpec{Name: "p1", Region: "a", Zone: "1"}}
	testNodepoolAutoScalerFail   = &DynamicNodePool{Name: "Test", ServerType: "s1", Image: "ubuntu", StorageDiskSize: newIntP(50), Count: 1, AutoscalerConfig: AutoscalerConfig{Min: 1, Max: 3}, ProviderSpec: ProviderSpec{Name: "p1", Region: "a", Zone: "1"}}
	testDomainFail               = &Manifest{
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{Name: "VERY-LONG-NAME-FOR-CLUSTER", Pools: Pool{
					Control: []string{"VERY-LONG-NAME-FOR-NODE-POOL1-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
					Compute: []string{"VERY-LONG-NAME-FOR-NODE-POOL2-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
				}},
			},
		},
		NodePools: NodePool{
			Dynamic: []DynamicNodePool{
				{Name: "VERY-LONG-NAME-FOR-NODE-POOL1-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Count: 10},
				{Name: "VERY-LONG-NAME-FOR-NODE-POOL2-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Count: 100},
			},
		},
	}

	testProxyFailOffMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "Off",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testProxyFailOnMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "On",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testProxyFailNothingMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testProxyFailDefaultMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "Default",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testProxyPassOnMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "on",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testProxyPassOffMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "off",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testProxyPassDefaultMode = &Kubernetes{
		Clusters: []Cluster{
			{Name: "CLUSTER", Pools: Pool{
				Control: []string{"np1"},
			},
				Network: "10.0.0.0/8",
				Version: "v1.29.0",
				InstallationProxy: &InstallationProxy{
					Mode:     "default",
					Endpoint: "http://proxy.claudie.io:8880",
				}},
		},
	}

	testDomainSuccess = &Manifest{
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{Name: "CLUSTER", Pools: Pool{
					Control: []string{"POOL-1"},
					Compute: []string{"POOL-2"},
				}},
			},
		},
		NodePools: NodePool{
			Dynamic: []DynamicNodePool{
				{Name: "POOL-1", Count: 10},
				{Name: "POOL-2", Count: 100},
			},
		},
	}

	testK8s = &Manifest{
		Name: "foo",
		Providers: Provider{
			Hetzner: []Hetzner{{
				Name:        "foo",
				Credentials: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Templates: &TemplateRepository{
					Repository: "test.com",
					Path:       "/subset/dir",
				},
			},
			},
		},
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{
					Name:    "foooo",
					Version: "v1.29.2",
					Network: "192.168.1.0/24",
					Pools: Pool{
						Control: []string{"control-1", "control-2"},
						Compute: []string{"compute-1", "compute-2"},
					},
				},
			},
		},
		NodePools: NodePool{
			Dynamic: []DynamicNodePool{
				{Name: "control-1", Count: 10, ServerType: "small", Image: "ubuntu", ProviderSpec: ProviderSpec{Name: "foo", Region: "north", Zone: "1"}},
				{Name: "compute-1", Count: 100, ServerType: "small", Image: "ubuntu", ProviderSpec: ProviderSpec{Name: "foo", Region: "north", Zone: "1"}},
			},
			Static: []StaticNodePool{
				{
					Name: "control-2",
				}, {
					Name: "compute-2",
					Taints: []k8sV1.Taint{
						{
							Key:    "test",
							Value:  "foo",
							Effect: "NoExecute",
						},
						{
							Key:    "test",
							Effect: "NoExecute",
						},
					},
					Labels: map[string]string{
						"test":              "foo",
						"test1":             "bar",
						"claudie.io/test-1": "success",
					},
				},
			},
		},
	}

	testNpDiskSizeSuccessfulNoDisk  = DynamicNodePool{Name: "control-1", Count: 10, ServerType: "small", Image: "ubuntu", ProviderSpec: ProviderSpec{Name: "foo", Region: "north", Zone: "1"}, StorageDiskSize: newIntP(0)}
	testNpDiskSizeSuccessfulFifty   = DynamicNodePool{Name: "control-1", Count: 10, ServerType: "small", Image: "ubuntu", ProviderSpec: ProviderSpec{Name: "foo", Region: "north", Zone: "1"}, StorageDiskSize: newIntP(50)}
	testNpDiskSizeSuccessfulDefault = DynamicNodePool{Name: "control-1", Count: 10, ServerType: "small", Image: "ubuntu", ProviderSpec: ProviderSpec{Name: "foo", Region: "north", Zone: "1"}}
	testNpDiskSizeSuccessfulFail    = DynamicNodePool{Name: "control-1", Count: 10, ServerType: "small", Image: "ubuntu", ProviderSpec: ProviderSpec{Name: "foo", Region: "north", Zone: "1"}, StorageDiskSize: newIntP(10)}
)

func newIntP(a int32) *int32 {
	return &a
}

func TestLoadBalancerRoles(t *testing.T) {
	ci := spectesting.GenerateFakeK8SClusterInfo(true, "192.168.0.0/16", "10.1.0.0/16")
	roles := spectesting.GenerateFakeRoles(false, ci)
	loadbalancer := &LoadBalancer{}

	for _, r := range roles {
		loadbalancer.Roles = append(loadbalancer.Roles, Role{
			Name:        r.Name,
			Protocol:    r.Protocol,
			Port:        int32(rand.IntN(ReservedPortRangeStart)),
			TargetPort:  r.TargetPort,
			TargetPools: generics.RemoveDuplicates(r.TargetPools),
			Settings: &RoleSettings{
				ProxyProtocol:  r.Settings.ProxyProtocol,
				StickySessions: r.Settings.StickySessions,
			},
			EnvoyProxy: &EnvoyProxy{
				Cds: r.Settings.EnvoyCds,
				Lds: r.Settings.EnvoyLds,
			},
		})
	}

	m := Manifest{}
	for _, n := range ci.NodePools {
		m.NodePools.Dynamic = append(m.NodePools.Dynamic, DynamicNodePool{
			Name: n.Name,
		})
	}

	err := loadbalancer.Validate(&m)
	require.NoError(t, err)

	loadbalancer.Roles[0].Port = ReservedPortRangeStart
	for {
		err = loadbalancer.Validate(&m)
		require.Error(t, err)

		loadbalancer.Roles[0].Port += 1
		if loadbalancer.Roles[0].Port == ReservedPortRangeEnd {
			break
		}
	}
}

// TestDomain tests the domain which will be formed from node name
func TestDomain(t *testing.T) {
	err := CheckLengthOfFutureDomain(testDomainSuccess)
	require.NoError(t, err)
	err = CheckLengthOfFutureDomain(testDomainFail)
	require.Error(t, err)
}

// TestKubernetes tests the kubernetes version validation
func TestKubernetes(t *testing.T) {
	err := testClusterVersionPass.Validate(testManifest)
	require.NoError(t, err)
	err = testClusterVersionFailMajor.Validate(testManifest)
	require.Error(t, err)
	err = testClusterVersionFailMinor.Validate(testManifest)
	require.Error(t, err)
}

func TestProxy(t *testing.T) {
	err := testProxyFailOffMode.Validate(testManifest)
	require.Error(t, err)
	err = testProxyFailOnMode.Validate(testManifest)
	require.Error(t, err)
	err = testProxyFailNothingMode.Validate(testManifest)
	require.Error(t, err)
	err = testProxyFailDefaultMode.Validate(testManifest)
	require.Error(t, err)
	err = testProxyPassDefaultMode.Validate(testManifest)
	require.NoError(t, err)
	err = testProxyPassOnMode.Validate(testManifest)
	require.NoError(t, err)
	err = testProxyPassOffMode.Validate(testManifest)
	require.NoError(t, err)
}

// TestNodepool tests the nodepool spec validation
func TestNodepool(t *testing.T) {
	err := testNodepoolAutoScalerSuccAC.Validate(&Manifest{})
	require.NoError(t, err)
	err = testNodepoolAutoScalerSucc.Validate(&Manifest{})
	require.NoError(t, err)
	err = testNodepoolAutoScalerFail.Validate(&Manifest{})
	require.Error(t, err)
}

// TestNodepool tests the nodepool spec validation for dynamic and static node pools.
func TestNodepools(t *testing.T) {
	err := testK8s.Validate()
	require.NoError(t, err)
}

// TestStorageDiskSize tests the storageDiskSize validation.
func TestStorageDiskSize(t *testing.T) {
	r := require.New(t)
	r.NoError(testNpDiskSizeSuccessfulNoDisk.Validate(&Manifest{}))
	r.NoError(testNpDiskSizeSuccessfulFifty.Validate(&Manifest{}))
	r.NoError(testNpDiskSizeSuccessfulDefault.Validate(&Manifest{}))
	r.Error(testNpDiskSizeSuccessfulFail.Validate(&Manifest{}))
}
