package service

import (
	"fmt"
	"testing"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/stretchr/testify/assert"

	"google.golang.org/protobuf/proto"
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
		into *store.Config
	}
	tests := []struct {
		name     string
		args     args
		wantErr  assert.ErrorAssertionFunc
		validate func(t *testing.T, into *store.Config)
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
				},
				into: &store.Config{
					Version:  0,
					Name:     "ok-overwrite-existing-lbs",
					K8SCtx:   store.KubernetesContext{},
					Manifest: store.Manifest{},
					Clusters: map[string]*store.ClusterState{
						"test-cluster": {
							Desired: store.Clusters{
								LoadBalancers: func() []byte {
									lbs := &spec.LoadBalancers{Clusters: []*spec.LBcluster{
										{
											ClusterInfo: &spec.ClusterInfo{Name: "lb-test-1", Hash: "hash"},
											Roles: []*spec.Role{{
												Name:        "ingress",
												Protocol:    "tcp",
												Port:        6447,
												TargetPort:  6447,
												TargetPools: []string{"worker"},
												RoleType:    spec.RoleType_Ingress,
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
									}}
									b, err := proto.Marshal(lbs)
									assert.Nil(t, err)
									return b
								}(),
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into *store.Config) {
				var m spec.LoadBalancers
				assert.Nil(t, proto.Unmarshal(into.Clusters["test-cluster"].Desired.LoadBalancers, &m))
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
				into: &store.Config{
					Version:  0,
					Name:     "ok-overwrite-existing-lbs",
					K8SCtx:   store.KubernetesContext{},
					Manifest: store.Manifest{},
					Clusters: map[string]*store.ClusterState{
						"test-cluster": {
							Desired: store.Clusters{
								LoadBalancers: func() []byte {
									lbs := &spec.LoadBalancers{Clusters: []*spec.LBcluster{
										{
											ClusterInfo: &spec.ClusterInfo{Name: "lb-test-1", Hash: "hash"},
											Roles: []*spec.Role{{
												Name:        "ingress",
												Protocol:    "tcp",
												Port:        6447,
												TargetPort:  6447,
												TargetPools: []string{"worker"},
												RoleType:    spec.RoleType_Ingress,
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
									}}
									b, err := proto.Marshal(lbs)
									assert.Nil(t, err)
									return b
								}(),
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into *store.Config) {
				var m spec.LoadBalancers
				assert.Nil(t, proto.Unmarshal(into.Clusters["test-cluster"].Desired.LoadBalancers, &m))
				assert.Equal(t, 0, len(m.Clusters))

				assert.Nil(t, into.Clusters["test-k8s"])
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
				into: &store.Config{
					Version:  0,
					Name:     "ok-overwrite-existing-lbs",
					K8SCtx:   store.KubernetesContext{},
					Manifest: store.Manifest{},
					Clusters: map[string]*store.ClusterState{
						"test-k8s": {},
						"test-cluster": {
							Desired: store.Clusters{
								LoadBalancers: func() []byte {
									lbs := &spec.LoadBalancers{Clusters: []*spec.LBcluster{
										{
											ClusterInfo: &spec.ClusterInfo{Name: "lb-test-1", Hash: "hash"},
											Roles: []*spec.Role{{
												Name:        "ingress",
												Protocol:    "tcp",
												Port:        6447,
												TargetPort:  6447,
												TargetPools: []string{"worker"},
												RoleType:    spec.RoleType_Ingress,
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
									}}
									b, err := proto.Marshal(lbs)
									assert.Nil(t, err)
									return b
								}(),
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into *store.Config) {
				var m spec.LoadBalancers
				assert.Nil(t, proto.Unmarshal(into.Clusters["test-cluster"].Desired.LoadBalancers, &m))
				assert.Equal(t, 0, len(m.Clusters))

				var m2 spec.LoadBalancers
				assert.Nil(t, proto.Unmarshal(into.Clusters["test-k8s"].Desired.LoadBalancers, &m2))
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
			tt.wantErr(t, createLBClustersFromManifest(tt.args.from, tt.args.into), fmt.Sprintf("createLBClustersFromManifest(%v, %v)", tt.args.from, tt.args.into))
			tt.validate(t, tt.args.into)
		})
	}
}

func Test_createK8sClustersFromManifest(t *testing.T) {
	type args struct {
		from *manifest.Manifest
		into *store.Config
	}
	tests := []struct {
		name     string
		args     args
		wantErr  assert.ErrorAssertionFunc
		validate func(t *testing.T, into *store.Config)
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
				into: &store.Config{
					Name: "catch-deleted-cluster",
					Clusters: map[string]*store.ClusterState{
						"test-cluster": {
							Current: store.Clusters{
								K8s:           []byte("random"),
								LoadBalancers: []byte("random"),
							},
							Desired: store.Clusters{
								K8s:           []byte("random"),
								LoadBalancers: []byte("random"),
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into *store.Config) {
				assert.NotNil(t, into.Clusters["test-cluster"])
				assert.Nil(t, into.Clusters["test-cluster"].Desired.K8s)
				assert.Nil(t, into.Clusters["test-cluster"].Desired.LoadBalancers)
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
				into: &store.Config{
					Name: "catch-deleted-cluster",
					Clusters: map[string]*store.ClusterState{
						"k8s-cluster": {
							Current: store.Clusters{
								K8s:           []byte("random"),
								LoadBalancers: []byte("random"),
							},
							Desired: store.Clusters{
								K8s:           []byte("random"),
								LoadBalancers: []byte("random"),
							},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, into *store.Config) {
				assert.NotNil(t, into.Clusters["k8s-cluster"])
				assert.Equal(t, []byte("random"), into.Clusters["k8s-cluster"].Desired.LoadBalancers)

				var m spec.K8Scluster
				assert.Nil(t, proto.Unmarshal(into.Clusters["k8s-cluster"].Desired.K8s, &m))
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
			tt.wantErr(t, createK8sClustersFromManifest(tt.args.from, tt.args.into), fmt.Sprintf("createK8sClustersFromManifest(%v, %v)", tt.args.from, tt.args.into))
			tt.validate(t, tt.args.into)
		})
	}
}
