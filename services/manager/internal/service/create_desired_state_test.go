package service

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/spectesting"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func Test_getRolesAttachedToLBCluster(t *testing.T) {
	type args struct {
		roles     []manifest.Role
		roleNames []string
	}
	tests := []struct {
		name string
		args args
		want []*spec.Role
	}{
		{
			name: "lb-roles-empty",
			args: args{
				roles:     []manifest.Role{},
				roleNames: []string{},
			},
			want: nil,
		},
		{
			name: "lb-roles-api-server",
			args: args{
				roles: []manifest.Role{
					{
						Name:        "test",
						Protocol:    "tcp",
						Port:        6443,
						TargetPort:  6443,
						TargetPools: []string{"control"},
						Settings: &manifest.RoleSettings{
							ProxyProtocol:  true,
							StickySessions: true,
						},
					},
				},
				roleNames: []string{"test"},
			},
			want: []*spec.Role{
				{
					Name:        "test",
					Protocol:    "tcp",
					Port:        6443,
					TargetPort:  6443,
					TargetPools: []string{"control"},
					RoleType:    spec.RoleType_ApiServer,
					Settings: &spec.Role_Settings{
						ProxyProtocol:  true,
						StickySessions: true,
						EnvoyAdminPort: -1,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tt.want, getRolesAttachedToLBCluster(tt.args.roles, tt.args.roleNames), "getRolesAttachedToLBCluster(%v, %v)", tt.args.roles, tt.args.roleNames)
		})
	}
}

func Test_getDNS(t *testing.T) {
	type args struct {
		dns  manifest.DNS
		from *manifest.Manifest
	}
	tests := []struct {
		name    string
		args    args
		want    *spec.DNS
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "getDNS-no-zone",
			args:    args{dns: manifest.DNS{}, from: &manifest.Manifest{Name: "test"}},
			want:    nil,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.NotNil(t, err) },
		},
		{
			name: "getDNS-ok",
			args: args{
				dns: manifest.DNS{
					DNSZone:  "test-zone",
					Provider: "test-provider",
					Hostname: "test-hostname",
				},
				from: &manifest.Manifest{
					Name: "test",
					Providers: manifest.Provider{
						HetznerDNS: []manifest.HetznerDNS{
							{Name: "test-provider", ApiToken: "test-token", Templates: &manifest.TemplateRepository{
								Repository: "https://github.com/berops/claudie-config",
								Tag:        strPtr("v0.1.2"),
								Path:       "/templates/terraformer/gcp",
							}},
						},
					},
				},
			},
			want: &spec.DNS{
				DnsZone:  "test-zone",
				Hostname: "test-hostname",
				Provider: &spec.Provider{
					SpecName:          "test-provider",
					CloudProviderName: "hetznerdns",
					ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
					Templates: &spec.TemplateRepository{
						Repository: "https://github.com/berops/claudie-config",
						Tag:        strPtr("v0.1.2"),
						Path:       "/templates/terraformer/gcp",
						CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b",
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getDNS(tt.args.dns, tt.args.from)
			if !tt.wantErr(t, err, fmt.Sprintf("getDNS(%v, %v)", tt.args.dns, tt.args.from)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getDNS(%v, %v)", tt.args.dns, tt.args.from)
		})
	}
}

func Test_createLBClustersFromManifest(t *testing.T) {
	type args struct {
		from *manifest.Manifest
		into map[string]*spec.Clusters
	}
	tests := []struct {
		name     string
		args     args
		wantErr  assert.ErrorAssertionFunc
		validate func(t *testing.T, into map[string]*spec.Clusters)
	}{
		{
			name: "ok-overwrite-existing-lbs",
			args: args{
				from: &manifest.Manifest{
					NodePools: manifest.NodePool{
						Dynamic: []manifest.DynamicNodePool{{
							Name:  "lb-pool",
							Count: 3,
							ProviderSpec: manifest.ProviderSpec{
								Name:   "test-provider",
								Region: "earth",
								Zone:   "europe",
							},
						}},
					},
					Name: "ok-overwrite-existing-lbs",
					Providers: manifest.Provider{
						HetznerDNS: []manifest.HetznerDNS{
							{Name: "test-provider", ApiToken: "test-token", Templates: &manifest.TemplateRepository{
								Repository: "https://github.com/berops/claudie-config",
								Path:       "/templates/terraformer/gcp",
							}},
						},
					},
					LoadBalancer: manifest.LoadBalancer{
						Roles: []manifest.Role{
							{
								Name:        "kubeapi",
								Protocol:    "tcp",
								Port:        6443,
								TargetPort:  6443,
								TargetPools: []string{"control"},
								Settings: &manifest.RoleSettings{
									ProxyProtocol:  true,
									StickySessions: false,
								},
							},
						},
						Clusters: []manifest.LoadBalancerCluster{
							{
								Name:        "test-lb-cluster",
								Roles:       []string{"kubeapi"},
								DNS:         manifest.DNS{DNSZone: "test-zone", Provider: "test-provider", Hostname: "test-hostname"},
								TargetedK8s: "test-cluster",
								Pools:       []string{"lb-pool"},
							},
						},
					},
					Kubernetes: manifest.Kubernetes{},
				},
				into: map[string]*spec.Clusters{
					"test-cluster": {
						LoadBalancers: &spec.LoadBalancers{
							Clusters: []*spec.LBcluster{
								{
									ClusterInfo: &spec.ClusterInfo{Name: "lb-test-1", Hash: "hash"},
									Roles: []*spec.Role{{
										Name:        "ingress",
										Protocol:    "tcp",
										Port:        6447,
										TargetPort:  6447,
										TargetPools: []string{"worker"},
										RoleType:    spec.RoleType_Ingress,
										Settings: &spec.Role_Settings{
											ProxyProtocol:  true,
											StickySessions: false,
										},
									}},
									Dns: &spec.DNS{
										DnsZone:  "test-zone",
										Hostname: "test-hostname-worker",
										Provider: &spec.Provider{
											SpecName:          "test-provider",
											CloudProviderName: "hetznerdns",
											ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
											Templates:         &spec.TemplateRepository{},
										},
									},
									TargetedK8S: "test-cluster",
								},
								{
									ClusterInfo: &spec.ClusterInfo{Name: "lb-test-2", Hash: "hash"},
									Roles: []*spec.Role{{
										Name:        "kubeapi",
										Protocol:    "tcp",
										Port:        6443,
										TargetPort:  6443,
										TargetPools: []string{"control"},
										RoleType:    spec.RoleType_ApiServer,
										Settings: &spec.Role_Settings{
											ProxyProtocol:  true,
											StickySessions: false,
										},
									}},
									Dns: &spec.DNS{
										DnsZone:  "test-zone",
										Hostname: "test-hostname",
										Provider: &spec.Provider{
											SpecName:          "test-provider",
											CloudProviderName: "hetznerdns",
											ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
											Templates:         &spec.TemplateRepository{},
										},
									},
									TargetedK8S: "test-cluster",
								},
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into map[string]*spec.Clusters) {
				m := into["test-cluster"].LoadBalancers
				assert.Equal(t, 1, len(m.Clusters))
				assert.Equal(t, "test-lb-cluster", m.Clusters[0].ClusterInfo.Name)
				// 0 as dynamic nodes are created at a later step.
				assert.Equal(t, 0, len(m.Clusters[0].ClusterInfo.NodePools[0].Nodes))
			},
		},
		{
			name: "ok-overwrite-existing-lbs-2",
			args: args{
				from: &manifest.Manifest{
					NodePools: manifest.NodePool{
						Dynamic: []manifest.DynamicNodePool{{
							Name:  "lb-pool",
							Count: 2,
							ProviderSpec: manifest.ProviderSpec{
								Name:   "test-provider",
								Region: "earth",
								Zone:   "europe",
							},
						}},
					},
					Name: "ok-overwrite-existing-lbs",
					Providers: manifest.Provider{
						HetznerDNS: []manifest.HetznerDNS{
							{Name: "test-provider", ApiToken: "test-token", Templates: &manifest.TemplateRepository{
								Repository: "https://github.com/berops/claudie-config",
								Path:       "/templates/terraformer/gcp",
							}},
						},
					},
					LoadBalancer: manifest.LoadBalancer{
						Roles: []manifest.Role{
							{
								Name:        "kubeapi",
								Protocol:    "tcp",
								Port:        6443,
								TargetPort:  6443,
								TargetPools: []string{"control"},
								Settings: &manifest.RoleSettings{
									ProxyProtocol:  true,
									StickySessions: true,
								},
							},
						},
						Clusters: []manifest.LoadBalancerCluster{
							{
								Name:        "test-lb-cluster",
								Roles:       []string{"kubeapi"},
								DNS:         manifest.DNS{DNSZone: "test-zone", Provider: "test-provider", Hostname: "test-hostname"},
								TargetedK8s: "test-k8s",
								Pools:       []string{"lb-pool"},
							},
						},
					},
				},
				into: map[string]*spec.Clusters{
					"test-cluster": {
						LoadBalancers: &spec.LoadBalancers{Clusters: []*spec.LBcluster{
							{
								ClusterInfo: &spec.ClusterInfo{Name: "lb-test-1", Hash: "hash"},
								Roles: []*spec.Role{{
									Name:        "ingress",
									Protocol:    "tcp",
									Port:        6447,
									TargetPort:  6447,
									TargetPools: []string{"worker"},
									RoleType:    spec.RoleType_Ingress,
									Settings: &spec.Role_Settings{
										ProxyProtocol:  true,
										StickySessions: true,
									},
								}},
								Dns: &spec.DNS{
									DnsZone:  "test-zone",
									Hostname: "test-hostname-worker",
									Provider: &spec.Provider{
										SpecName:          "test-provider",
										CloudProviderName: "hetznerdns",
										ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
										Templates:         &spec.TemplateRepository{},
									},
								},
								TargetedK8S: "test-cluster",
							},
							{
								ClusterInfo: &spec.ClusterInfo{Name: "lb-test-2", Hash: "hash"},
								Roles: []*spec.Role{{
									Name:        "kubeapi",
									Protocol:    "tcp",
									Port:        6443,
									TargetPort:  6443,
									TargetPools: []string{"control"},
									RoleType:    spec.RoleType_ApiServer,
									Settings: &spec.Role_Settings{
										ProxyProtocol:  true,
										StickySessions: true,
									},
								}},
								Dns: &spec.DNS{
									DnsZone:  "test-zone",
									Hostname: "test-hostname",
									Provider: &spec.Provider{
										SpecName:          "test-provider",
										CloudProviderName: "hetznerdns",
										ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
										Templates:         &spec.TemplateRepository{},
									},
								},
								TargetedK8S: "test-cluster",
							},
						}},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into map[string]*spec.Clusters) {
				m := into["test-cluster"].LoadBalancers
				assert.Equal(t, 2, len(m.Clusters))

				assert.Nil(t, into["test-k8s"])
			},
		},
		{
			name: "ok-overwrite-existing-lbs-3",
			args: args{
				from: &manifest.Manifest{
					NodePools: manifest.NodePool{
						Dynamic: []manifest.DynamicNodePool{{
							Name:  "lb-pool",
							Count: 3,
							ProviderSpec: manifest.ProviderSpec{
								Name:   "test-provider",
								Region: "earth",
								Zone:   "europe",
							},
						}},
					},
					Name: "ok-overwrite-existing-lbs",
					Providers: manifest.Provider{
						HetznerDNS: []manifest.HetznerDNS{
							{Name: "test-provider", ApiToken: "test-token", Templates: &manifest.TemplateRepository{
								Repository: "https://github.com/berops/claudie-config",
								Path:       "/templates/terraformer/gcp",
							}},
						},
					},
					LoadBalancer: manifest.LoadBalancer{
						Roles: []manifest.Role{
							{
								Name:        "kubeapi",
								Protocol:    "tcp",
								Port:        6443,
								TargetPort:  6443,
								TargetPools: []string{"control"},
								Settings: &manifest.RoleSettings{
									ProxyProtocol:  true,
									StickySessions: true,
								},
							},
						},
						Clusters: []manifest.LoadBalancerCluster{
							{
								Name:        "test-lb-cluster",
								Roles:       []string{"kubeapi"},
								DNS:         manifest.DNS{DNSZone: "test-zone", Provider: "test-provider", Hostname: "test-hostname"},
								TargetedK8s: "test-k8s",
								Pools:       []string{"lb-pool"},
							},
						},
					},
				},
				into: map[string]*spec.Clusters{
					"test-k8s": {},
					"test-cluster": {
						LoadBalancers: &spec.LoadBalancers{
							Clusters: []*spec.LBcluster{
								{
									ClusterInfo: &spec.ClusterInfo{Name: "lb-test-1", Hash: "hash"},
									Roles: []*spec.Role{{
										Name:        "ingress",
										Protocol:    "tcp",
										Port:        6447,
										TargetPort:  6447,
										TargetPools: []string{"worker"},
										RoleType:    spec.RoleType_Ingress,
										Settings: &spec.Role_Settings{
											ProxyProtocol:  true,
											StickySessions: true,
										},
									}},
									Dns: &spec.DNS{
										DnsZone:  "test-zone",
										Hostname: "test-hostname-worker",
										Provider: &spec.Provider{
											SpecName:          "test-provider",
											CloudProviderName: "hetznerdns",
											ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
											Templates:         &spec.TemplateRepository{},
										},
									},
									TargetedK8S: "test-cluster",
								},
								{
									ClusterInfo: &spec.ClusterInfo{Name: "lb-test-2", Hash: "hash"},
									Roles: []*spec.Role{{
										Name:        "kubeapi",
										Protocol:    "tcp",
										Port:        6443,
										TargetPort:  6443,
										TargetPools: []string{"control"},
										RoleType:    spec.RoleType_ApiServer,
										Settings: &spec.Role_Settings{
											ProxyProtocol:  true,
											StickySessions: true,
										},
									}},
									Dns: &spec.DNS{
										DnsZone:  "test-zone",
										Hostname: "test-hostname",
										Provider: &spec.Provider{
											SpecName:          "test-provider",
											CloudProviderName: "hetznerdns",
											ProviderType:      &spec.Provider_Hetznerdns{Hetznerdns: &spec.HetznerDNSProvider{Token: "test-token"}},
											Templates:         &spec.TemplateRepository{},
										},
									},
									TargetedK8S: "test-cluster",
								},
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into map[string]*spec.Clusters) {
				m := into["test-cluster"].LoadBalancers
				assert.Equal(t, 2, len(m.Clusters))

				m2 := into["test-k8s"].LoadBalancers
				assert.Equal(t, 1, len(m2.Clusters))
				assert.Equal(t, "test-lb-cluster", m2.Clusters[0].ClusterInfo.Name)
				// 0 as dynamic nodes are created at a later step.
				assert.Equal(t, 0, len(m2.Clusters[0].ClusterInfo.NodePools[0].Nodes))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.wantErr(t, createLBClustersFromManifest(tt.args.from, &tt.args.into), fmt.Sprintf("createLBClustersFromManifest(%v, %v)", tt.args.from, tt.args.into))
			tt.validate(t, tt.args.into)
		})
	}
}

func Test_createK8sClustersFromManifest(t *testing.T) {
	type args struct {
		from *manifest.Manifest
		into map[string]*spec.Clusters
	}
	tests := []struct {
		name     string
		args     args
		wantErr  assert.ErrorAssertionFunc
		validate func(t *testing.T, into map[string]*spec.Clusters)
	}{
		{
			name: "catch-deleted-cluster",
			args: args{
				from: &manifest.Manifest{
					Name: "catch-deleted-cluster",
					NodePools: manifest.NodePool{
						Dynamic: []manifest.DynamicNodePool{{
							Name:  "pool",
							Count: 4,
							ProviderSpec: manifest.ProviderSpec{
								Name:   "test-provider",
								Region: "earth",
								Zone:   "europe",
							},
						}},
					},
					Providers: manifest.Provider{
						HetznerDNS: []manifest.HetznerDNS{
							{Name: "test-provider", ApiToken: "test-token", Templates: &manifest.TemplateRepository{
								Repository: "https://github.com/berops/claudie-config",
								Path:       "/templates/terraformer/gcp",
							}},
						},
					},
					Kubernetes: manifest.Kubernetes{
						Clusters: []manifest.Cluster{
							{
								Name:    "k8s-cluster",
								Version: "1.29.0",
								Network: "192.168.0.1/24",
								Pools: manifest.Pool{
									Control: []string{"pool"},
									Compute: []string{"pool"},
								},
							},
						},
					},
				},
				into: map[string]*spec.Clusters{
					"test-cluster": {
						K8S:           &spec.K8Scluster{},
						LoadBalancers: &spec.LoadBalancers{},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into map[string]*spec.Clusters) {
				assert.Nil(t, into["test-cluster"])
			},
		},
		{
			name: "update-existing-cluster",
			args: args{
				from: &manifest.Manifest{
					Name: "catch-deleted-cluster",
					NodePools: manifest.NodePool{
						Dynamic: []manifest.DynamicNodePool{{
							Name:  "pool",
							Count: 4,
							ProviderSpec: manifest.ProviderSpec{
								Name:   "test-provider",
								Region: "earth",
								Zone:   "europe",
							},
						}},
					},
					Providers: manifest.Provider{
						HetznerDNS: []manifest.HetznerDNS{
							{Name: "test-provider", ApiToken: "test-token", Templates: &manifest.TemplateRepository{
								Repository: "https://github.com/berops/claudie-config",
								Path:       "/templates/terraformer/gcp",
							}},
						},
					},
					Kubernetes: manifest.Kubernetes{
						Clusters: []manifest.Cluster{
							{
								Name:    "k8s-cluster",
								Version: "1.29.0",
								Network: "192.168.0.1/24",
								Pools: manifest.Pool{
									Control: []string{"pool"},
									Compute: []string{"pool"},
								},
							},
						},
					},
				},
				into: map[string]*spec.Clusters{
					"k8s-cluster": &spec.Clusters{
						K8S:           &spec.K8Scluster{},
						LoadBalancers: &spec.LoadBalancers{},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into map[string]*spec.Clusters) {
				assert.NotNil(t, into["k8s-cluster"])

				m := into["k8s-cluster"].K8S
				assert.Equal(t, "k8s-cluster", m.ClusterInfo.Name)
				assert.Equal(t, "1.29.0", m.Kubernetes)
				assert.Equal(t, "192.168.0.1/24", m.Network)
				assert.Equal(t, 2, len(m.ClusterInfo.NodePools))
				assert.True(t, m.ClusterInfo.NodePools[0].IsControl)
				assert.False(t, m.ClusterInfo.NodePools[1].IsControl)
				assert.Equal(t, "pool", m.ClusterInfo.NodePools[0].Name)
				assert.Equal(t, "pool", m.ClusterInfo.NodePools[1].Name)

				// 0 as dynamic nodes are created at a later step.
				assert.Equal(t, 0, len(m.ClusterInfo.NodePools[0].Nodes))
				assert.Equal(t, 0, len(m.ClusterInfo.NodePools[1].Nodes))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.wantErr(t, createK8sClustersFromManifest(tt.args.from, &tt.args.into), fmt.Sprintf("createK8sClustersFromManifest(%v, %v)", tt.args.from, tt.args.into))
			tt.validate(t, tt.args.into)
		})
	}
}

func Test_calculateCIDR(t *testing.T) {
	type args struct {
		baseCIDR  string
		key       string
		exits     map[string][]string
		nodepools []*spec.DynamicNodePool
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantCidrs []string
	}{
		{
			name: "test-01",
			args: args{
				baseCIDR: baseSubnetCIDR,
				key:      "test-01",
				exits:    map[string][]string{"test-01": {}},
				nodepools: []*spec.DynamicNodePool{
					{Cidr: ""},
					{Cidr: ""},
					{Cidr: ""},
				},
			},
			wantErr: false,
			wantCidrs: []string{
				"10.0.0.0/24",
				"10.0.1.0/24",
				"10.0.2.0/24",
			},
		},
		{
			name: "test-02",
			args: args{
				baseCIDR: baseSubnetCIDR,
				key:      "test-02",
				exits:    map[string][]string{"test-02": {}},
				nodepools: []*spec.DynamicNodePool{
					{Cidr: "10.0.0.0/24"},
					{Cidr: "10.0.2.0/24"},
				},
			},
			wantErr: false,
			wantCidrs: []string{
				"10.0.0.0/24",
				"10.0.2.0/24",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := calculateCIDR(tt.args.baseCIDR, tt.args.key, tt.args.exits, tt.args.nodepools); (err != nil) != tt.wantErr {
				t.Errorf("calculateCIDR() error = %v, wantErr %v", err, tt.wantErr)
			}
			for i, cidr := range tt.wantCidrs {
				if tt.args.nodepools[i].Cidr != cidr {
					t.Errorf("calculateCIDR() error = %v want %v", tt.args.nodepools[i].Cidr, cidr)
				}
			}
		})
	}
}

func TestGetCIDR(t *testing.T) {
	type testCase struct {
		desc     string
		baseCIDR string
		position int
		existing []string
		out      string
	}

	testDataSucc := []testCase{
		{
			desc:     "Second octet change",
			baseCIDR: "10.0.0.0/24",
			position: 1,
			existing: []string{"10.1.0.0/24"},
			out:      "10.0.0.0/24",
		},
		{
			desc:     "Third octet change",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			existing: []string{"10.0.0.0/24"},
			out:      "10.0.1.0/24",
		},
	}
	for _, test := range testDataSucc {
		if out, err := getCIDR(test.baseCIDR, test.position, test.existing); out != test.out || err != nil {
			t.Error(test.desc, err, out)
		}
	}
	testDataFail := []testCase{
		{
			desc:     "Max IP error",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			existing: func() []string {
				var m []string
				for i := range 256 {
					m = append(m, fmt.Sprintf("10.0.%d.0/24", i))
				}
				return m
			}(),
			out: "",
		},
		{
			desc:     "Invalid base CIDR",
			baseCIDR: "300.0.0.0/24",
			position: 2,
			existing: []string{"10.0.0.0/24"},
			out:      "10.0.10.0/24",
		},
	}
	for _, test := range testDataFail {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			if _, err := getCIDR(test.baseCIDR, test.position, test.existing); err == nil {
				t.Error(test.desc, "test should have failed, but was successful")
			} else {
				t.Log(err)
			}
		})
	}
}

func Test_fillMissingCIDR2(t *testing.T) {
	t.Parallel()

	var (
		k8s     = spectesting.GenerateFakeK8SCluster(true)
		lbs1    = spectesting.GenerateFakeLBCluster(false, k8s.ClusterInfo)
		lbs2    = spectesting.GenerateFakeLBCluster(true, k8s.ClusterInfo)
		desired = &spec.Clusters{
			K8S: k8s,
			LoadBalancers: &spec.LoadBalancers{
				Clusters: []*spec.LBcluster{lbs1, lbs2},
			},
		}
	)
	// 1. remove all generated cidrs. an
	for _, np := range nodepools.ExtractDynamic(desired.K8S.ClusterInfo.NodePools) {
		np.Cidr = ""
	}
	for _, c := range desired.LoadBalancers.Clusters {
		for _, np := range nodepools.ExtractDynamic(c.ClusterInfo.NodePools) {
			np.Cidr = ""
		}
	}

	// 2. narrow down the spec name and provider region.
	regions := []string{"a", "b", "c"}
	specName := []string{"g", "h", "i"}
	for _, np := range nodepools.ExtractDynamic(desired.K8S.ClusterInfo.NodePools) {
		np.Provider.SpecName = specName[rand.IntN(len(specName))]
		np.Region = regions[rand.IntN(len(specName))]
	}
	for _, c := range desired.LoadBalancers.Clusters {
		for _, np := range nodepools.ExtractDynamic(c.ClusterInfo.NodePools) {
			np.Provider.SpecName = specName[rand.IntN(len(specName))]
			np.Region = regions[rand.IntN(len(specName))]
		}
	}

	// 3. generate cidrs
	if err := generateMissingCIDR(nil, desired); err != nil {
		t.Errorf("failed to generate CIDRs: %fv", err)
	}

	// 4. each np within a cluster should have a different cidr.
	existing := make(map[string][]string)
	for p, np := range nodepools.ByProviderRegion(desired.K8S.ClusterInfo.NodePools) {
		assert.True(t, len(np) > 0)
		for _, np := range nodepools.ExtractDynamic(np) {
			assert.NotEmpty(t, np.Cidr)
			assert.False(t, slices.Contains(existing[p], np.Cidr))
			existing[p] = append(existing[p], np.Cidr)
		}
	}
	for _, c := range desired.LoadBalancers.Clusters {
		clear(existing)
		for p, np := range nodepools.ByProviderRegion(c.ClusterInfo.NodePools) {
			assert.True(t, len(np) > 0)
			for _, np := range nodepools.ExtractDynamic(np) {
				assert.NotEmpty(t, np.Cidr)
				assert.False(t, slices.Contains(existing[p], np.Cidr))
				existing[p] = append(existing[p], np.Cidr)
			}
		}
	}
}

func Test_fillMissingCIDR(t *testing.T) {
	type args struct {
		c *spec.Clusters
		d *spec.Clusters
	}
	tests := []struct {
		name     string
		args     args
		wantErr  bool
		validate func(t *testing.T, args args)
	}{
		{
			name: "test01",
			args: args{
				c: &spec.Clusters{
					K8S: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
						Name: "k8s-current-test-01",
						Hash: "01",
						NodePools: []*spec.NodePool{
							{
								Name: "k8s-01",
								Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
									Cidr:     "10.0.0.0/24",
									Region:   "europe-west",
									Provider: &spec.Provider{SpecName: "hetzner-1", CloudProviderName: "hetzner"},
								}},
							},
							{
								Name: "k8s-02",
								Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
									Cidr:     "10.0.1.0/24",
									Region:   "europe-east",
									Provider: &spec.Provider{SpecName: "gcp-1", CloudProviderName: "gcp"},
								}},
							},
						}}},
					LoadBalancers: &spec.LoadBalancers{Clusters: []*spec.LBcluster{
						{
							ClusterInfo: &spec.ClusterInfo{
								Name: "lb-current-test-01",
								Hash: "03",
								NodePools: []*spec.NodePool{
									{
										Name: "lb-01",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.0.0/24",
											Region:   "europe-east",
											Provider: &spec.Provider{SpecName: "gcp-1", CloudProviderName: "gcp"},
										}},
									},
									{
										Name: "lb-02",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.1.0/24",
											Region:   "europe-east",
											Provider: &spec.Provider{SpecName: "gcp-1", CloudProviderName: "gcp"},
										}},
									},
								}},
						},
						{
							ClusterInfo: &spec.ClusterInfo{
								Name: "lb-current-test-02",
								Hash: "04",
								NodePools: []*spec.NodePool{
									{
										Name: "lb-02-01",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.0.0/24",
											Region:   "europe-north",
											Provider: &spec.Provider{SpecName: "oci-1", CloudProviderName: "oci"},
										}},
									},
									{
										Name: "lb-02-02",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.1.0/24",
											Region:   "europe-north",
											Provider: &spec.Provider{SpecName: "oci-2", CloudProviderName: "oci"},
										}},
									},
								}},
						},
					}},
				},
				d: &spec.Clusters{
					K8S: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
						Name: "k8s-current-test-01",
						Hash: "01",
						NodePools: []*spec.NodePool{
							{
								Name: "k8s-01",
								Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
									Cidr:     "10.0.0.0/24",
									Region:   "europe-west",
									Provider: &spec.Provider{SpecName: "hetzner-1", CloudProviderName: "hetzner"},
								}},
							},
							{
								Name: "k8s-03",
								Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
									Cidr:     "",
									Region:   "europe-west",
									Provider: &spec.Provider{SpecName: "hetzner-1", CloudProviderName: "hetzner"},
								}},
							},
						}}},
					LoadBalancers: &spec.LoadBalancers{Clusters: []*spec.LBcluster{
						{
							ClusterInfo: &spec.ClusterInfo{
								Name: "lb-current-test-01",
								Hash: "03",
								NodePools: []*spec.NodePool{
									{
										Name: "lb-01",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.0.0/24",
											Region:   "europe-east",
											Provider: &spec.Provider{SpecName: "gcp-1", CloudProviderName: "gcp"},
										}},
									},
									{
										Name: "lb-03",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "",
											Region:   "europe-east",
											Provider: &spec.Provider{SpecName: "gcp-1", CloudProviderName: "gcp"},
										}},
									},
								}},
						},
						{
							ClusterInfo: &spec.ClusterInfo{
								Name: "lb-current-test-02",
								Hash: "04",
								NodePools: []*spec.NodePool{
									{
										Name: "lb-02-01",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.0.0/24",
											Region:   "europe-north",
											Provider: &spec.Provider{SpecName: "oci-1", CloudProviderName: "oci"},
										}},
									},
									{
										Name: "lb-02-02",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "10.0.1.0/24",
											Region:   "europe-north",
											Provider: &spec.Provider{SpecName: "oci-2", CloudProviderName: "oci"},
										}},
									},
								}},
						},
						{
							ClusterInfo: &spec.ClusterInfo{
								Name: "lb-current-test-03",
								Hash: "04",
								NodePools: []*spec.NodePool{
									{
										Name: "lb-02-01",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "",
											Region:   "europe-north",
											Provider: &spec.Provider{SpecName: "oci-1", CloudProviderName: "oci"},
										}},
									},
									{
										Name: "lb-02-02",
										Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
											Cidr:     "",
											Region:   "europe-north",
											Provider: &spec.Provider{SpecName: "oci-2", CloudProviderName: "oci"},
										}},
									},
								}},
						},
					}},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, args args) {
				assert.Equal(t, "10.0.1.0/24", args.d.K8S.ClusterInfo.NodePools[1].GetDynamicNodePool().Cidr)
				assert.Equal(t, "10.0.2.0/24", args.d.LoadBalancers.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Cidr)
				assert.Equal(t, "10.0.0.0/24", args.d.LoadBalancers.Clusters[2].ClusterInfo.NodePools[0].GetDynamicNodePool().Cidr)
				assert.Equal(t, "10.0.0.0/24", args.d.LoadBalancers.Clusters[2].ClusterInfo.NodePools[1].GetDynamicNodePool().Cidr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := generateMissingCIDR(tt.args.c, tt.args.d); (err != nil) != tt.wantErr {
				t.Errorf("fillMissingCIDR() = %v want = %v", err, tt.wantErr)
			}
			tt.validate(t, tt.args)
		})
	}
}

func Test_generateMissingDynamicNode(t *testing.T) {
	t.Parallel()

	// control nodepool
	np := &spec.NodePool{
		Name:      "test",
		Nodes:     make([]*spec.Node, 0, 2),
		IsControl: true,
		Type: &spec.NodePool_DynamicNodePool{
			DynamicNodePool: &spec.DynamicNodePool{
				Count: 4,
			},
		},
	}

	PopulateDynamicNodes("testing-1", np)
	assert.Equal(t, 4, len(np.Nodes))
	assert.Equal(t, "testing-1-test-03", np.Nodes[2].Name)
	assert.Equal(t, "testing-1-test-04", np.Nodes[3].Name)
	assert.Equal(t, spec.NodeType_master, np.Nodes[2].NodeType)
	assert.Equal(t, spec.NodeType_master, np.Nodes[3].NodeType)

	// worker nodepool
	np = &spec.NodePool{
		Name:      "worker-test",
		Nodes:     make([]*spec.Node, 0, 2),
		IsControl: false,
		Type: &spec.NodePool_DynamicNodePool{
			DynamicNodePool: &spec.DynamicNodePool{
				Count: 3,
			},
		},
	}

	PopulateDynamicNodes("testing-2", np)
	assert.Equal(t, 3, len(np.Nodes))
	assert.Equal(t, "testing-2-worker-test-03", np.Nodes[2].Name)
	assert.Equal(t, spec.NodeType_worker, np.Nodes[2].NodeType)

	np = &spec.NodePool{
		Name:      "tt",
		Nodes:     []*spec.Node{{Name: "testing-2-tt-02"}},
		IsControl: false,
		Type: &spec.NodePool_DynamicNodePool{
			DynamicNodePool: &spec.DynamicNodePool{
				Count: 3,
			},
		},
	}

	PopulateDynamicNodes("testing-2", np)

	assert.Equal(t, 3, len(np.Nodes))
	assert.Equal(t, "testing-2-tt-02", np.Nodes[0].Name)
	assert.Equal(t, "testing-2-tt-01", np.Nodes[1].Name)
	assert.Equal(t, "testing-2-tt-03", np.Nodes[2].Name)
}

func Test_generateClaudieReservedPorts(t *testing.T) {
	t.Parallel()

	ports := generateClaudieReservedPorts()
	assert.True(t, len(ports) == (manifest.MaxRolesPerLoadBalancer+manifest.AdditionalReservedPorts))
	beg, end := manifest.ReservedPortRangeStart, manifest.ReservedPortRangeEnd
	for beg < end {
		assert.Equal(t, beg, ports[beg-manifest.ReservedPortRangeStart])
		beg++
	}
	assert.Equal(t, int(math.MaxUint16), ports[len(ports)-1])
}

func Test_FillMissingEnvoyAdminPorts(t *testing.T) {
	t.Parallel()

	const lbcount = 3

	var lbs []*spec.LBcluster
	for range lbcount {
		k8s := spectesting.GenerateFakeK8SCluster(true)
		lb := spectesting.GenerateFakeLBCluster(false, k8s.ClusterInfo)
		lbs = append(lbs, lb)
	}

	PopulateEnvoyAdminPorts(&spec.Clusters{
		LoadBalancers: &spec.LoadBalancers{
			Clusters: lbs,
		},
	})

	// assert no duplicates
	for _, lb := range lbs {
		seen := make(map[int32]struct{})
		assert.True(t, len(lb.Roles) > 1)
		for _, r := range lb.Roles {
			_, ok := seen[r.Settings.EnvoyAdminPort]
			assert.True(t, r.Settings.EnvoyAdminPort >= 0)
			assert.False(t, ok)
			seen[r.Settings.EnvoyAdminPort] = struct{}{}
		}
	}

	// test limits
	size := manifest.MaxRolesPerLoadBalancer
	roles := make([]*spec.Role, size)
	for i := range roles {
		roles[i] = &spec.Role{
			Name: fmt.Sprint(i),
			Settings: &spec.Role_Settings{
				EnvoyAdminPort: -1,
			},
		}
		assert.True(t, roles[i].Settings.EnvoyAdminPort == -1)
	}

	// 3 loadbalancers each with the maximum amount of allowed roles
	lbs = lbs[:0]
	for range lbcount {
		roles := slices.Clone(roles)
		lbs = append(lbs, &spec.LBcluster{
			Roles: roles,
		})
	}

	PopulateEnvoyAdminPorts(&spec.Clusters{
		LoadBalancers: &spec.LoadBalancers{
			Clusters: lbs,
		},
	})

	// assert no duplicates
	for _, lb := range lbs {
		seen := make(map[int32]struct{})
		assert.True(t, len(lb.Roles) > 1)
		for _, r := range lb.Roles {
			_, ok := seen[r.Settings.EnvoyAdminPort]
			assert.True(t, r.Settings.EnvoyAdminPort >= 0)
			assert.False(t, ok)
			seen[r.Settings.EnvoyAdminPort] = struct{}{}
		}
	}

	size = manifest.MaxRolesPerLoadBalancer
	roles = make([]*spec.Role, size)
	used := make(map[int32]struct{})
	for i := range roles {
		roles[i] = &spec.Role{
			Name: fmt.Sprint(i),
			Settings: &spec.Role_Settings{
				EnvoyAdminPort: -1,
			},
		}
		if rand.Int()%2 == 0 {
			for {
				next := int32(rand.IntN(math.MaxUint16))
				if _, ok := used[next]; ok {
					continue
				}
				used[next] = struct{}{}
				roles[i].Settings.EnvoyAdminPort = next
				break
			}
		}
	}

	// 3 loadbalancers each with the maximum amount of allowed roles
	lbs = lbs[:0]
	for range lbcount {
		roles := slices.Clone(roles)
		lbs = append(lbs, &spec.LBcluster{
			Roles: roles,
		})
	}

	PopulateEnvoyAdminPorts(&spec.Clusters{
		LoadBalancers: &spec.LoadBalancers{
			Clusters: lbs,
		},
	})

	// assert no duplicates
	for _, lb := range lbs {
		seen := make(map[int32]struct{})
		assert.True(t, len(lb.Roles) > 1)
		for _, r := range lb.Roles {
			_, ok := seen[r.Settings.EnvoyAdminPort]
			assert.True(t, r.Settings.EnvoyAdminPort >= 0)
			assert.False(t, ok)
			seen[r.Settings.EnvoyAdminPort] = struct{}{}
		}
	}
}

func Test_fixupAutoscalerCounts(t *testing.T) {
	k8s := spectesting.GenerateFakeK8SCluster(true)

	clusters := &spec.Clusters{
		K8S:           k8s,
		LoadBalancers: &spec.LoadBalancers{},
	}

	var wantMarkedForDeletion int32
	var markedForDeletion int32

	for _, n := range nodepools.Autoscaled(k8s.ClusterInfo.NodePools) {
		dyn := n.GetDynamicNodePool()
		if dyn.AutoscalerConfig.TargetSize < dyn.Count {
			wantMarkedForDeletion += dyn.Count - dyn.AutoscalerConfig.TargetSize
		}
		for _, n := range n.Nodes {
			if n.Status == spec.NodeStatus_MarkedForDeletion {
				markedForDeletion += 1
			}
		}
	}

	assert.Equal(t, markedForDeletion, int32(0))

	fixupAutoscalerCounts(clusters)

	for _, n := range nodepools.Autoscaled(k8s.ClusterInfo.NodePools) {
		for _, n := range n.Nodes {
			if n.Status == spec.NodeStatus_MarkedForDeletion {
				markedForDeletion += 1
			}
		}
	}

	assert.Equal(t, markedForDeletion, wantMarkedForDeletion)
}
