package service

import (
	"fmt"
	"github.com/berops/claudie/internal/manifest"
	"testing"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func Test_transferExistingDns(t *testing.T) {
	type args struct {
		current *spec.LoadBalancers
		desired *spec.LoadBalancers
	}
	tests := []struct {
		name     string
		args     args
		validate func(t *testing.T, args args)
	}{
		{
			name:     "desired-nil",
			args:     args{current: &spec.LoadBalancers{}, desired: nil},
			validate: func(t *testing.T, args args) { assert.Nil(t, args.desired) },
		},
		{
			name: "generate-hostname-1",
			args: args{
				current: &spec.LoadBalancers{},
				desired: &spec.LoadBalancers{
					Clusters: []*spec.LBcluster{
						{
							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
							Dns:         &spec.DNS{Hostname: ""},
						},
					},
				},
			},
			validate: func(t *testing.T, args args) {
				assert.NotEmpty(t, args.desired.Clusters[0].Dns.Hostname)
			},
		},
		{
			name: "generate-hostname-2",
			args: args{
				current: &spec.LoadBalancers{
					Clusters: []*spec.LBcluster{
						{
							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
							Dns:         &spec.DNS{Hostname: "test-hostname"},
						},
					},
				},
				desired: &spec.LoadBalancers{
					Clusters: []*spec.LBcluster{
						{
							ClusterInfo: &spec.ClusterInfo{Name: "cluster-1"},
							Dns:         &spec.DNS{Hostname: ""},
						},
					},
				},
			},
			validate: func(t *testing.T, args args) {
				assert.NotEmpty(t, args.desired.Clusters[0].Dns.Hostname)
				assert.Equal(t, "test-hostname", args.desired.Clusters[0].Dns.Hostname)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			transferExistingDns(tt.args.current, tt.args.desired)
			tt.validate(t, tt.args)
		})
	}
}

func Test_updateClusterInfo(t *testing.T) {
	type args struct {
		desired *spec.ClusterInfo
		current *spec.ClusterInfo
	}
	tests := []struct {
		name     string
		args     args
		wantErr  assert.ErrorAssertionFunc
		validate func(t *testing.T, args args)
	}{
		{
			name: "transfer-cluster-info-state",
			args: args{
				current: &spec.ClusterInfo{
					Name: "current",
					Hash: "current",
					NodePools: []*spec.NodePool{
						{
							NodePoolType: &spec.NodePool_DynamicNodePool{
								DynamicNodePool: &spec.DynamicNodePool{
									PublicKey:  "current-pk",
									PrivateKey: "current-sk",
									Cidr:       "current-cidr",
								},
							},
							Name: "np0",
							Nodes: []*spec.Node{
								{
									Name:     "node-0",
									Private:  "private",
									Public:   "public",
									NodeType: spec.NodeType_apiEndpoint,
									Username: "username",
								},
							},
							IsControl: true,
						},
					},
				},
				desired: &spec.ClusterInfo{
					Name: "current",
					Hash: "desired",
					NodePools: []*spec.NodePool{
						{Name: "np0", NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, args args) {
				assert.Equal(t, 1, len(args.desired.NodePools))
				assert.Equal(t, 1, len(args.desired.NodePools[0].Nodes))
				assert.Equal(t, "current-cidr", args.desired.NodePools[0].GetDynamicNodePool().Cidr)
				assert.Equal(t, "current-pk", args.desired.NodePools[0].GetDynamicNodePool().PublicKey)
				assert.Equal(t, "current-sk", args.desired.NodePools[0].GetDynamicNodePool().PrivateKey)
			},
		},
		{
			name: "transfer-cluster-info-state-static-pool",
			args: args{
				current: &spec.ClusterInfo{
					Name: "current",
					Hash: "current",
					NodePools: []*spec.NodePool{
						{
							NodePoolType: &spec.NodePool_StaticNodePool{
								StaticNodePool: &spec.StaticNodePool{},
							},
							Name: "np0",
							Nodes: []*spec.Node{
								{
									Name:     "node-0",
									Private:  "private",
									Public:   "127.0.0.1",
									NodeType: spec.NodeType_worker,
									Username: "username",
								},
							},
							IsControl: false,
						},
					},
				},
				desired: &spec.ClusterInfo{
					Name: "current",
					Hash: "desired",
					NodePools: []*spec.NodePool{
						{
							Name:         "np0",
							NodePoolType: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}},
							Nodes:        []*spec.Node{{Public: "127.0.0.1"}},
						},
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, args args) {
				assert.Equal(t, 1, len(args.desired.NodePools))
				assert.Equal(t, 1, len(args.desired.NodePools[0].Nodes))
				assert.Equal(t, "node-0", args.desired.NodePools[0].Nodes[0].Name)
				assert.Equal(t, "private", args.desired.NodePools[0].Nodes[0].Private)
				assert.Equal(t, spec.NodeType_worker, args.desired.NodePools[0].Nodes[0].NodeType)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.wantErr(t, updateClusterInfo(tt.args.desired, tt.args.current), fmt.Sprintf("updateClusterInfo(%v, %v)", tt.args.desired, tt.args.current))
			tt.validate(t, tt.args)
		})
	}
}

func Test_copyK8sNodePoolsNamesFromCurrentState(t *testing.T) {
	type args struct {
		used     map[string]struct{}
		nodepool string
		current  *spec.K8Scluster
		desired  *spec.K8Scluster
	}
	tests := []struct {
		name     string
		args     args
		validate func(t *testing.T, args args)
	}{
		{
			name: "transfer-hashes",
			args: args{
				used: map[string]struct{}{
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
				},
				nodepool: "np0",
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					NodePools: []*spec.NodePool{{Name: fmt.Sprintf("np0-%s", utils.CreateHash(utils.HashLength))}},
				}},
				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					NodePools: []*spec.NodePool{{Name: "np0"}},
				}},
			},
			validate: func(t *testing.T, args args) {
				assert.Equal(t, args.current.ClusterInfo.NodePools[0].Name, args.desired.ClusterInfo.NodePools[0].Name)
				_, hash := utils.GetNameAndHashFromNodepool("np0", args.current.ClusterInfo.NodePools[0].Name)
				_, ok := args.used[hash]
				assert.True(t, ok)
			},
		},
		{
			name: "no-transfer",
			args: args{
				used: map[string]struct{}{
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
				},
				nodepool: "np0",
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					NodePools: []*spec.NodePool{{Name: fmt.Sprintf("np0-%s", utils.CreateHash(utils.HashLength))}},
				}},
				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					NodePools: []*spec.NodePool{{Name: "np-0"}},
				}},
			},
			validate: func(t *testing.T, args args) {
				assert.NotEqual(t, args.current.ClusterInfo.NodePools[0].Name, args.desired.ClusterInfo.NodePools[0].Name)
				_, hash := utils.GetNameAndHashFromNodepool("np0", args.current.ClusterInfo.NodePools[0].Name)
				assert.Equal(t, utils.HashLength, len(hash))
				_, ok := args.used[hash]
				assert.False(t, ok)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			copyK8sNodePoolsNamesFromCurrentState(tt.args.used, tt.args.nodepool, tt.args.current, tt.args.desired)
			tt.validate(t, tt.args)
		})
	}
}

func Test_copyLbNodePoolNamesFromCurrentState(t *testing.T) {
	type args struct {
		used     map[string]struct{}
		nodepool string
		current  []*spec.LBcluster
		desired  []*spec.LBcluster
	}
	tests := []struct {
		name     string
		args     args
		validate func(t *testing.T, args args)
	}{
		{
			name: "transfer-hash",
			args: args{
				used: map[string]struct{}{
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
				},
				nodepool: "np-0",
				current: []*spec.LBcluster{{
					ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
						{Name: fmt.Sprintf("np-0-%s", utils.CreateHash(utils.HashLength))},
					}},
				}},
				desired: []*spec.LBcluster{{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{{Name: "np-0"}}}}},
			},
			validate: func(t *testing.T, args args) {
				assert.Equal(t, args.current[0].ClusterInfo.NodePools[0].Name, args.desired[0].ClusterInfo.NodePools[0].Name)
				_, hash := utils.GetNameAndHashFromNodepool("np-0", args.desired[0].ClusterInfo.NodePools[0].Name)
				assert.Equal(t, utils.HashLength, len(hash))
				_, ok := args.used[hash]
				assert.True(t, ok)
			},
		},
		{
			name: "no-transfer",
			args: args{
				used: map[string]struct{}{
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
					utils.CreateHash(utils.HashLength): {},
				},
				nodepool: "np-0",
				current: []*spec.LBcluster{{
					ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
						{Name: fmt.Sprintf("np-0-%s", utils.CreateHash(utils.HashLength))},
					}},
				}},
				desired: []*spec.LBcluster{{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{{Name: "np0"}}}}},
			},
			validate: func(t *testing.T, args args) {
				assert.NotEqual(t, args.current[0].ClusterInfo.NodePools[0].Name, args.desired[0].ClusterInfo.NodePools[0].Name)
				_, hash := utils.GetNameAndHashFromNodepool("np-0", args.desired[0].ClusterInfo.NodePools[0].Name)
				assert.Empty(t, hash)
				_, ok := args.used[hash]
				assert.False(t, ok)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			copyLbNodePoolNamesFromCurrentState(tt.args.used, tt.args.nodepool, tt.args.current, tt.args.desired)
			tt.validate(t, tt.args)
		})
	}
}

func Test_deduplicateNodepoolNames(t *testing.T) {
	type args struct {
		from  *manifest.Manifest
		state *spec.ClusterState
	}
	tests := []struct {
		name     string
		args     args
		validate func(t *testing.T, args args)
	}{
		{
			name: "dedup-k8s",
			args: args{
				from: &manifest.Manifest{
					NodePools: manifest.NodePool{Dynamic: []manifest.DynamicNodePool{{Name: "np-0"}}},
				},
				state: &spec.ClusterState{
					Desired: &spec.Clusters{
						K8S: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
							Name: "desired",
							NodePools: []*spec.NodePool{
								{Name: "np-0", NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
								{Name: "np-0", NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
							},
						}},
					},
				},
			},
			validate: func(t *testing.T, args args) {
				name, hash := utils.GetNameAndHashFromNodepool("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[0].Name)
				assert.Equal(t, utils.HashLength, len(hash))
				assert.Equal(t, "np-0", name)

				name, hash = utils.GetNameAndHashFromNodepool("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[1].Name)
				assert.Equal(t, utils.HashLength, len(hash))
				assert.Equal(t, "np-0", name)
			},
		},
		{
			name: "dedup-k8s-with-lbs",
			args: args{
				from: &manifest.Manifest{
					NodePools: manifest.NodePool{Dynamic: []manifest.DynamicNodePool{{Name: "np-0"}}},
				},
				state: &spec.ClusterState{
					Desired: &spec.Clusters{
						K8S: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
							Name: "desired",
							NodePools: []*spec.NodePool{
								{Name: "np-0", NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
								{Name: "np-0", NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
							},
						}},
						LoadBalancers: &spec.LoadBalancers{Clusters: []*spec.LBcluster{{ClusterInfo: &spec.ClusterInfo{
							Name: "desired-lb",
							NodePools: []*spec.NodePool{
								{Name: "np-0", NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
							},
						}}}},
					},
				},
			},
			validate: func(t *testing.T, args args) {
				name, hash1 := utils.GetNameAndHashFromNodepool("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[0].Name)
				assert.Equal(t, utils.HashLength, len(hash1))
				assert.Equal(t, "np-0", name)

				name, hash2 := utils.GetNameAndHashFromNodepool("np-0", args.state.Desired.K8S.ClusterInfo.NodePools[1].Name)
				assert.Equal(t, utils.HashLength, len(hash2))
				assert.Equal(t, "np-0", name)

				name, hash3 := utils.GetNameAndHashFromNodepool("np-0", args.state.Desired.LoadBalancers.Clusters[0].ClusterInfo.NodePools[0].Name)
				assert.Equal(t, utils.HashLength, len(hash3))
				assert.Equal(t, "np-0", name)

				assert.NotEqual(t, hash1, hash2)
				assert.NotEqual(t, hash1, hash3)
				assert.NotEqual(t, hash2, hash3)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deduplicateNodepoolNames(tt.args.from, tt.args.state)
		})
	}
}
