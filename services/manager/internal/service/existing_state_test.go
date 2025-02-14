package service

// import (
// 	"fmt"
// 	"testing"

// 	"github.com/berops/claudie/internal/hash"
// 	"github.com/berops/claudie/internal/manifest"
// 	"github.com/berops/claudie/internal/nodepools"
// 	"github.com/berops/claudie/proto/pb/spec"
// 	"github.com/stretchr/testify/assert"
// )

// func Test_transferExistingDns(t *testing.T) {
// 	type args struct {
// 		current *spec.LoadBalancers
// 		desired *spec.LoadBalancers
// 	}
// 	tests := []struct {
// 		name     string
// 		args     args
// 		validate func(t *testing.T, args args)
// 	}{
// 		{
// 			name:     "desired-nil",
// 			args:     args{current: &spec.LoadBalancers{}, desired: nil},
// 			validate: func(t *testing.T, args args) { assert.Nil(t, args.desired) },
// 		},
// 		{
// 			name: "generate-hostname-1",
// 			args: args{
// 				current: &spec.LoadBalancers{},
// 				desired: &spec.LoadBalancers{
// 					Clusters: []*spec.LBcluster{
// 						{
// 							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
// 							Dns:         &spec.DNS{Hostname: ""},
// 						},
// 					},
// 				},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.NotEmpty(t, args.desired.Clusters[0].Dns.Hostname)
// 			},
// 		},
// 		{
// 			name: "generate-hostname-2",
// 			args: args{
// 				current: &spec.LoadBalancers{
// 					Clusters: []*spec.LBcluster{
// 						{
// 							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
// 							Dns:         &spec.DNS{Hostname: "test-hostname", Endpoint: "test-endpoint"},
// 						},
// 					},
// 				},
// 				desired: &spec.LoadBalancers{
// 					Clusters: []*spec.LBcluster{
// 						{
// 							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
// 							Dns:         &spec.DNS{Hostname: ""},
// 						},
// 					},
// 				},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.NotEmpty(t, args.desired.Clusters[0].Dns.Hostname)
// 				assert.Equal(t, "test-hostname", args.desired.Clusters[0].Dns.Hostname)
// 				assert.Equal(t, "test-endpoint", args.desired.Clusters[0].Dns.Endpoint)
// 			},
// 		},
// 		{
// 			name: "generate-hostname-2",
// 			args: args{
// 				current: &spec.LoadBalancers{
// 					Clusters: []*spec.LBcluster{
// 						{
// 							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
// 							Dns:         &spec.DNS{Hostname: "test-hostname", Endpoint: "test-endpoint"},
// 						},
// 					},
// 				},
// 				desired: &spec.LoadBalancers{
// 					Clusters: []*spec.LBcluster{
// 						{
// 							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
// 							Dns:         &spec.DNS{Hostname: "other-hostname"},
// 						},
// 					},
// 				},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.NotEmpty(t, args.desired.Clusters[0].Dns.Hostname)
// 				assert.Equal(t, "other-hostname", args.desired.Clusters[0].Dns.Hostname)
// 				assert.Empty(t, args.desired.Clusters[0].Dns.Endpoint)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()
// 			transferExistingDns(tt.args.current, tt.args.desired)
// 			tt.validate(t, tt.args)
// 		})
// 	}
// }

// func Test_updateClusterInfo(t *testing.T) {
// 	type args struct {
// 		desired *spec.ClusterInfo
// 		current *spec.ClusterInfo
// 	}
// 	tests := []struct {
// 		name     string
// 		args     args
// 		wantErr  assert.ErrorAssertionFunc
// 		validate func(t *testing.T, args args)
// 	}{
// 		{
// 			name: "transfer-cluster-info-state-autoscaler",
// 			args: args{
// 				current: &spec.ClusterInfo{
// 					Name: "current",
// 					Hash: "current",
// 					NodePools: []*spec.NodePool{
// 						{
// 							Type: &spec.NodePool_DynamicNodePool{
// 								DynamicNodePool: &spec.DynamicNodePool{
// 									PublicKey:  "current-pk",
// 									PrivateKey: "current-sk",
// 									Cidr:       "current-cidr",
// 									Count:      5,
// 									AutoscalerConfig: &spec.AutoscalerConf{
// 										Min: 3,
// 										Max: 12,
// 									},
// 								},
// 							},
// 							Name: "np0",
// 							Nodes: []*spec.Node{
// 								{
// 									Name:     "node-0",
// 									Private:  "private",
// 									Public:   "public",
// 									NodeType: spec.NodeType_apiEndpoint,
// 									Username: "username",
// 								},
// 								{
// 									Name:     "node-1",
// 									Private:  "private",
// 									Public:   "public",
// 									NodeType: spec.NodeType_worker,
// 									Username: "username",
// 								},
// 								{
// 									Name:     "node-2",
// 									Private:  "private",
// 									Public:   "public",
// 									NodeType: spec.NodeType_worker,
// 									Username: "username",
// 								},
// 							},
// 							IsControl: true,
// 						},
// 					},
// 				},
// 				desired: &spec.ClusterInfo{
// 					Name: "current",
// 					Hash: "desired",
// 					NodePools: []*spec.NodePool{
// 						{Name: "np0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{Count: 3, AutoscalerConfig: &spec.AutoscalerConf{
// 							Min: 3,
// 							Max: 12,
// 						}}}},
// 					},
// 				},
// 			},
// 			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
// 			validate: func(t *testing.T, args args) {
// 				assert.Equal(t, int32(5), args.desired.NodePools[0].GetDynamicNodePool().Count)
// 				assert.Equal(t, 1, len(args.desired.NodePools))
// 				assert.Equal(t, 5, len(args.desired.NodePools[0].Nodes))
// 				assert.Equal(t, "node-0", args.desired.NodePools[0].Nodes[0].Name)
// 				assert.Equal(t, "node-1", args.desired.NodePools[0].Nodes[1].Name)
// 				assert.Equal(t, "node-2", args.desired.NodePools[0].Nodes[2].Name)
// 				assert.Equal(t, "current-current-np0-01", args.desired.NodePools[0].Nodes[3].Name)
// 				assert.Equal(t, "current-current-np0-02", args.desired.NodePools[0].Nodes[4].Name)
// 				assert.Equal(t, "current-cidr", args.desired.NodePools[0].GetDynamicNodePool().Cidr)
// 				assert.Equal(t, "current-pk", args.desired.NodePools[0].GetDynamicNodePool().PublicKey)
// 				assert.Equal(t, "current-sk", args.desired.NodePools[0].GetDynamicNodePool().PrivateKey)
// 			},
// 		},
// 		{
// 			name: "transfer-cluster-info-state",
// 			args: args{
// 				current: &spec.ClusterInfo{
// 					Name: "current",
// 					Hash: "current",
// 					NodePools: []*spec.NodePool{
// 						{
// 							Type: &spec.NodePool_DynamicNodePool{
// 								DynamicNodePool: &spec.DynamicNodePool{
// 									PublicKey:  "current-pk",
// 									PrivateKey: "current-sk",
// 									Cidr:       "current-cidr",
// 									Count:      1,
// 								},
// 							},
// 							Name: "np0",
// 							Nodes: []*spec.Node{
// 								{
// 									Name:     "node-0",
// 									Private:  "private",
// 									Public:   "public",
// 									NodeType: spec.NodeType_apiEndpoint,
// 									Username: "username",
// 								},
// 							},
// 							IsControl: true,
// 						},
// 					},
// 				},
// 				desired: &spec.ClusterInfo{
// 					Name: "current",
// 					Hash: "desired",
// 					NodePools: []*spec.NodePool{
// 						{Name: "np0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{Count: 2}}},
// 					},
// 				},
// 			},
// 			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
// 			validate: func(t *testing.T, args args) {
// 				assert.Equal(t, 1, len(args.desired.NodePools))
// 				assert.Equal(t, 2, len(args.desired.NodePools[0].Nodes))
// 				assert.Equal(t, "node-0", args.desired.NodePools[0].Nodes[0].Name)
// 				assert.Equal(t, "current-current-np0-01", args.desired.NodePools[0].Nodes[1].Name)
// 				assert.Equal(t, "current-cidr", args.desired.NodePools[0].GetDynamicNodePool().Cidr)
// 				assert.Equal(t, "current-pk", args.desired.NodePools[0].GetDynamicNodePool().PublicKey)
// 				assert.Equal(t, "current-sk", args.desired.NodePools[0].GetDynamicNodePool().PrivateKey)
// 			},
// 		},
// 		{
// 			name: "transfer-cluster-info-state-static-pool",
// 			args: args{
// 				current: &spec.ClusterInfo{
// 					Name: "current",
// 					Hash: "current",
// 					NodePools: []*spec.NodePool{
// 						{
// 							Type: &spec.NodePool_StaticNodePool{
// 								StaticNodePool: &spec.StaticNodePool{},
// 							},
// 							Name: "np0",
// 							Nodes: []*spec.Node{
// 								{
// 									Name:     "node-0",
// 									Private:  "private",
// 									Public:   "127.0.0.1",
// 									NodeType: spec.NodeType_apiEndpoint,
// 									Username: "username",
// 								},
// 							},
// 							IsControl: false,
// 						},
// 					},
// 				},
// 				desired: &spec.ClusterInfo{
// 					Name: "current",
// 					Hash: "desired",
// 					NodePools: []*spec.NodePool{
// 						{
// 							Name: "np0",
// 							Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}},
// 							Nodes: []*spec.Node{
// 								{Public: "127.0.0.1"},
// 								{Public: "127.0.0.2"},
// 								{Public: "127.0.0.3"},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
// 			validate: func(t *testing.T, args args) {
// 				assert.Equal(t, 1, len(args.desired.NodePools))
// 				assert.Equal(t, 3, len(args.desired.NodePools[0].Nodes))
// 				assert.Equal(t, "node-0", args.desired.NodePools[0].Nodes[0].Name)
// 				assert.Equal(t, "private", args.desired.NodePools[0].Nodes[0].Private)
// 				assert.Equal(t, spec.NodeType_apiEndpoint, args.desired.NodePools[0].Nodes[0].NodeType)

// 				assert.Equal(t, "np0-01", args.desired.NodePools[0].Nodes[1].Name)
// 				assert.Equal(t, "np0-02", args.desired.NodePools[0].Nodes[2].Name)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()
// 			tt.wantErr(t, transferNodePools(tt.args.desired, tt.args.current), fmt.Sprintf("transferNodePools(%v, %v)", tt.args.desired, tt.args.current))
// 			tt.validate(t, tt.args)
// 		})
// 	}
// }

// func Test_copyK8sNodePoolsNamesFromCurrentState(t *testing.T) {
// 	type args struct {
// 		used     map[string]struct{}
// 		nodepool string
// 		current  *spec.K8Scluster
// 		desired  *spec.K8Scluster
// 	}
// 	tests := []struct {
// 		name     string
// 		args     args
// 		validate func(t *testing.T, args args)
// 	}{
// 		{
// 			name: "transfer-hashes",
// 			args: args{
// 				used: map[string]struct{}{
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 				},
// 				nodepool: "np0",
// 				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
// 					NodePools: []*spec.NodePool{{Name: fmt.Sprintf("np0-%s", hash.Create(hash.Length))}},
// 				}},
// 				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
// 					NodePools: []*spec.NodePool{{Name: "np0"}},
// 				}},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.Equal(t, args.current.ClusterInfo.NodePools[0].Name, args.desired.ClusterInfo.NodePools[0].Name)
// 				_, hash := nodepools.MatchNameAndHashWithTemplate("np0", args.current.ClusterInfo.NodePools[0].Name)
// 				_, ok := args.used[hash]
// 				assert.True(t, ok)
// 			},
// 		},
// 		{
// 			name: "no-transfer",
// 			args: args{
// 				used: map[string]struct{}{
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 				},
// 				nodepool: "np0",
// 				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
// 					NodePools: []*spec.NodePool{{Name: fmt.Sprintf("np0-%s", hash.Create(hash.Length))}},
// 				}},
// 				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
// 					NodePools: []*spec.NodePool{{Name: "np-0"}},
// 				}},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.NotEqual(t, args.current.ClusterInfo.NodePools[0].Name, args.desired.ClusterInfo.NodePools[0].Name)
// 				_, h := nodepools.MatchNameAndHashWithTemplate("np0", args.current.ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, hash.Length, len(h))
// 				_, ok := args.used[h]
// 				assert.False(t, ok)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()
// 			copyK8sNodePoolsNamesFromCurrentState(tt.args.used, tt.args.nodepool, tt.args.current, tt.args.desired)
// 			tt.validate(t, tt.args)
// 		})
// 	}
// }

// func Test_copyLbNodePoolNamesFromCurrentState(t *testing.T) {
// 	type args struct {
// 		used     map[string]struct{}
// 		nodepool string
// 		current  []*spec.LBcluster
// 		desired  []*spec.LBcluster
// 	}
// 	tests := []struct {
// 		name     string
// 		args     args
// 		validate func(t *testing.T, args args)
// 	}{
// 		{
// 			name: "transfer-hash",
// 			args: args{
// 				used: map[string]struct{}{
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 				},
// 				nodepool: "np-0",
// 				current: []*spec.LBcluster{{
// 					ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
// 						{Name: fmt.Sprintf("np-0-%s", hash.Create(hash.Length))},
// 					}},
// 				}},
// 				desired: []*spec.LBcluster{{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{{Name: "np-0"}}}}},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.Equal(t, args.current[0].ClusterInfo.NodePools[0].Name, args.desired[0].ClusterInfo.NodePools[0].Name)
// 				_, h := nodepools.MatchNameAndHashWithTemplate("np-0", args.desired[0].ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, hash.Length, len(h))
// 				_, ok := args.used[h]
// 				assert.True(t, ok)
// 			},
// 		},
// 		{
// 			name: "no-transfer",
// 			args: args{
// 				used: map[string]struct{}{
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 					hash.Create(hash.Length): {},
// 				},
// 				nodepool: "np-0",
// 				current: []*spec.LBcluster{{
// 					ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
// 						{Name: fmt.Sprintf("np-0-%s", hash.Create(hash.Length))},
// 					}},
// 				}},
// 				desired: []*spec.LBcluster{{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{{Name: "np0"}}}}},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				assert.NotEqual(t, args.current[0].ClusterInfo.NodePools[0].Name, args.desired[0].ClusterInfo.NodePools[0].Name)
// 				_, hash := nodepools.MatchNameAndHashWithTemplate("np-0", args.desired[0].ClusterInfo.NodePools[0].Name)
// 				assert.Empty(t, hash)
// 				_, ok := args.used[hash]
// 				assert.False(t, ok)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()
// 			copyLbNodePoolNamesFromCurrentState(tt.args.used, tt.args.nodepool, tt.args.current, tt.args.desired)
// 			tt.validate(t, tt.args)
// 		})
// 	}
// }

// func Test_deduplicateNodepoolNames(t *testing.T) {
// 	type args struct {
// 		from  *manifest.Manifest
// 		state *spec.ClusterState
// 	}
// 	tests := []struct {
// 		name     string
// 		args     args
// 		validate func(t *testing.T, args args)
// 	}{
// 		{
// 			name: "dedup-k8s",
// 			args: args{
// 				from: &manifest.Manifest{
// 					NodePools: manifest.NodePool{Dynamic: []manifest.DynamicNodePool{{Name: "np-0"}}},
// 				},
// 				state: &spec.ClusterState{
// 					Desired: &spec.Clusters{
// 						K8S: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
// 							Name: "desired",
// 							NodePools: []*spec.NodePool{
// 								{Name: "np-0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
// 								{Name: "np-0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
// 							},
// 						}},
// 					},
// 				},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				n, h := nodepools.MatchNameAndHashWithTemplate("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, hash.Length, len(h))
// 				assert.Equal(t, "np-0", n)

// 				n, h = nodepools.MatchNameAndHashWithTemplate("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[1].Name)
// 				assert.Equal(t, hash.Length, len(h))
// 				assert.Equal(t, "np-0", n)
// 			},
// 		},
// 		{
// 			name: "dedup-k8s-with-lbs",
// 			args: args{
// 				from: &manifest.Manifest{
// 					NodePools: manifest.NodePool{Dynamic: []manifest.DynamicNodePool{{Name: "np-0"}}},
// 				},
// 				state: &spec.ClusterState{
// 					Desired: &spec.Clusters{
// 						K8S: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
// 							Name: "desired",
// 							NodePools: []*spec.NodePool{
// 								{Name: "np-0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
// 								{Name: "np-0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
// 							},
// 						}},
// 						LoadBalancers: &spec.LoadBalancers{Clusters: []*spec.LBcluster{{ClusterInfo: &spec.ClusterInfo{
// 							Name: "desired-lb",
// 							NodePools: []*spec.NodePool{
// 								{Name: "np-0", Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
// 							},
// 						}}}},
// 					},
// 				},
// 			},
// 			validate: func(t *testing.T, args args) {
// 				name, hash1 := nodepools.MatchNameAndHashWithTemplate("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, hash.Length, len(hash1))
// 				assert.Equal(t, "np-0", name)

// 				name, hash2 := nodepools.MatchNameAndHashWithTemplate("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[1].Name)
// 				assert.Equal(t, hash.Length, len(hash2))
// 				assert.Equal(t, "np-0", name)

// 				name, hash3 := nodepools.MatchNameAndHashWithTemplate("np-0", args.state.Desired.LoadBalancers.Clusters[0].ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, hash.Length, len(hash3))
// 				assert.Equal(t, "np-0", name)

// 				assert.NotEqual(t, hash1, hash2)
// 				assert.NotEqual(t, hash1, hash3)
// 				assert.NotEqual(t, hash2, hash3)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			deduplicateNodepoolNames(tt.args.from, tt.args.state)
// 		})
// 	}
// }
