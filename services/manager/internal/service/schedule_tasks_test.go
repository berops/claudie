package service

import (
	"fmt"
	"testing"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var opts = cmpopts.IgnoreUnexported(
	spec.Config{},
	spec.KubernetesContext{},
	spec.Manifest{},
	spec.ClusterState{},
	spec.Clusters{},
	spec.Events{},
	spec.Workflow{},
	spec.K8Scluster{},
	spec.LoadBalancers{},
	spec.LBcluster{},
	spec.ClusterInfo{},
	spec.NodePool{},
	spec.Node{},
	spec.NodePool_DynamicNodePool{},
	spec.NodePool_StaticNodePool{},
	spec.DynamicNodePool{},
	spec.StaticNodePool{},
	spec.Provider{},
	spec.Provider_Hetzner{},
	spec.HetznerProvider{},
	spec.TemplateRepository{},
	spec.Task{},
	spec.TaskEvent{},
	timestamppb.Timestamp{},
	spec.RetryStrategy{},
)

func Test_findNewAPIEndpointCandidate(t *testing.T) {
	type args struct {
		desired []*spec.NodePool
	}
	tests := []struct {
		name         string
		args         args
		wantNodePool string
		wantNode     string
	}{
		{
			name: "find-candidate-ok",
			args: args{
				desired: []*spec.NodePool{
					{Name: "np-0", IsControl: false, Nodes: []*spec.Node{{Name: "0"}, {Name: "1"}}},
					{Name: "np-1", IsControl: true, Nodes: []*spec.Node{{Name: "3"}, {Name: "4"}}},
				},
			},
			wantNodePool: "np-1", wantNode: "3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			np, n := newAPIEndpointNodeCandidate(tt.args.desired)
			assert.Equalf(t, tt.wantNodePool, np, "findNewAPIEndpointCandidate(%v)", tt.args.desired)
			assert.Equalf(t, tt.wantNode, n, "findNewAPIEndpointCandidate(%v)", tt.args.desired)
		})
	}
}

func Test_deletedTargetApiNodePool(t *testing.T) {
	rnghash := utils.CreateHash(utils.HashLength)
	type args struct {
		k8sDiffResult nodePoolDiffResult
		current       *spec.K8Scluster
		currentLbs    []*spec.LBcluster
	}
	tests := []struct {
		name  string
		args  args
		want  []string
		want1 bool
	}{
		{
			name: "deleted-target-api-nodepools",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					deletedDynamic: map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
					deletedStatic:  map[string][]string{fmt.Sprintf("stat-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					NodePools: []*spec.NodePool{
						{Name: fmt.Sprintf("dyn-%s", rnghash), IsControl: true, Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
						{Name: fmt.Sprintf("stat-%s", rnghash), IsControl: true, Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}}},
					},
				}},
				currentLbs: []*spec.LBcluster{{
					Roles: []*spec.Role{
						{
							Name:        "api-server",
							Protocol:    "tcp",
							Port:        6443,
							TargetPort:  6443,
							TargetPools: []string{"dyn", "stat"},
							RoleType:    spec.RoleType_ApiServer,
						},
					},
					TargetedK8S: "current",
				}},
			},
			want:  []string{fmt.Sprintf("dyn-%s", rnghash), fmt.Sprintf("stat-%s", rnghash)},
			want1: true,
		},
		{
			name: "deleted-one-of-mane-api-nodepools",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					deletedDynamic: map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					NodePools: []*spec.NodePool{
						{Name: fmt.Sprintf("dyn-%s", rnghash), IsControl: true, Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
						{Name: fmt.Sprintf("stat-%s", rnghash), IsControl: true, Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}}},
					},
				}},
				currentLbs: []*spec.LBcluster{{
					Roles: []*spec.Role{
						{
							Name:        "api-server",
							Protocol:    "tcp",
							Port:        6443,
							TargetPort:  6443,
							TargetPools: []string{"dyn", "stat"},
							RoleType:    spec.RoleType_ApiServer,
						},
					},
					TargetedK8S: "current",
				}},
			},
			want:  nil,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, got1 := deletedTargetApiNodePools(tt.args.k8sDiffResult, tt.args.current, tt.args.currentLbs)
			assert.Equalf(t, tt.want, got, "deletedTargetApiNodePool(%v, %v, %v)", tt.args.k8sDiffResult, tt.args.current, tt.args.currentLbs)
			assert.Equalf(t, tt.want1, got1, "deletedTargetApiNodePool(%v, %v, %v)", tt.args.k8sDiffResult, tt.args.current, tt.args.currentLbs)
		})
	}
}

func Test_endpointNodePoolDeleted(t *testing.T) {
	rnghash := utils.CreateHash(utils.HashLength)
	type args struct {
		k8sDiffResult nodePoolDiffResult
		current       *spec.K8Scluster
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "ok-api-nodepool",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					deletedDynamic: map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
					deletedStatic:  map[string][]string{fmt.Sprintf("stat-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "test",
					Hash: "test",
					NodePools: []*spec.NodePool{{
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{{
							Name:     "1",
							NodeType: spec.NodeType_apiEndpoint,
						}},
						IsControl: true,
					}, {
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{{
							Name:     "1",
							NodeType: spec.NodeType_worker,
						}},
						IsControl: true,
					}},
				}},
			},
			want: true,
		},
		{
			name: "ok-api-nodepool-1",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					deletedDynamic: map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
					deletedStatic:  map[string][]string{fmt.Sprintf("stat-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "test",
					Hash: "test",
					NodePools: []*spec.NodePool{{
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{{
							Name:     "1",
							NodeType: spec.NodeType_worker,
						}},
						IsControl: true,
					}, {
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{{
							Name:     "1",
							NodeType: spec.NodeType_apiEndpoint,
						}},
						IsControl: true,
					}},
				}},
			},
			want: true,
		},
		{
			name: "no-api-nodepool",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					deletedDynamic: map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
					deletedStatic:  map[string][]string{fmt.Sprintf("stat-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "test",
					Hash: "test",
					NodePools: []*spec.NodePool{{
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{{
							Name:     "1",
							NodeType: spec.NodeType_worker,
						}},
						IsControl: true,
					}, {
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{{
							Name:     "1",
							NodeType: spec.NodeType_master,
						}},
						IsControl: true,
					}},
				}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tt.want, endpointNodeDeleted(tt.args.k8sDiffResult, tt.args.current), "endpointNodePoolDeleted(%v, %v)", tt.args.k8sDiffResult, tt.args.current)
		})
	}
}

func Test_craftK8sIR(t *testing.T) {
	rnghash := utils.CreateHash(utils.HashLength)
	type args struct {
		k8sDiffResult nodePoolDiffResult
		current       *spec.K8Scluster
		desired       *spec.K8Scluster
	}
	tests := []struct {
		name string
		args args
		want *spec.K8Scluster
	}{
		{
			name: "ok-includes-deleted",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					partialDeletedDynamic: map[string][]string{fmt.Sprintf("pdyn-%s", rnghash): {"2"}},
					partialDeletedStatic:  map[string][]string{fmt.Sprintf("pstat-%s", rnghash): {"2"}},
					deletedDynamic:        map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
					deletedStatic:         map[string][]string{fmt.Sprintf("stat-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					Hash: "hash",
					NodePools: []*spec.NodePool{{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-1",
								},
							},
						}},
						Name: fmt.Sprintf("pdyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.1", Public: "10.1", NodeType: spec.NodeType_apiEndpoint},
							{Name: "2", Private: "1.2", Public: "10.2", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: make(map[string]string)}},
						Name: fmt.Sprintf("pstat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.3", Public: "20.1", NodeType: spec.NodeType_master},
							{Name: "2", Private: "1.4", Public: "20.2", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-1",
								},
							},
						}},
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.5", Public: "10.3", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.6", Public: "10.4", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: make(map[string]string)}},
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.7", Public: "20.3", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.8", Public: "20.4", NodeType: spec.NodeType_worker}},
						IsControl: false,
					}},
				}},
				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					Hash: "hash",
					NodePools: []*spec.NodePool{{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 1,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-2",
								},
							},
						}},
						Name: fmt.Sprintf("pdyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.1", Public: "10.1", NodeType: spec.NodeType_apiEndpoint}},
						IsControl: true,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: make(map[string]string)}},
						Name: fmt.Sprintf("pstat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.3", Public: "20.1", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 3,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-3",
								},
							},
						}},
						Name: fmt.Sprintf("new-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.9", Public: "10.5", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.10", Public: "10.6", NodeType: spec.NodeType_worker},
							{Name: "3", Private: "1.11", Public: "10.7", NodeType: spec.NodeType_worker}},
						IsControl: false,
					}},
				}},
			},
			want: &spec.K8Scluster{
				ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					Hash: "hash",
					NodePools: []*spec.NodePool{{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-2",
								},
							},
						}},
						Name: fmt.Sprintf("pdyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.1", Public: "10.1", NodeType: spec.NodeType_apiEndpoint},
							{Name: "2", Private: "1.2", Public: "10.2", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}},
						Name: fmt.Sprintf("pstat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.3", Public: "20.1", NodeType: spec.NodeType_master},
							{Name: "2", Private: "1.4", Public: "20.2", NodeType: spec.NodeType_master},
						},
						IsControl: true}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 3,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-3",
								},
							},
						}},
						Name: fmt.Sprintf("new-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.9", Public: "10.5", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.10", Public: "10.6", NodeType: spec.NodeType_worker},
							{Name: "3", Private: "1.11", Public: "10.7", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-1",
								},
							},
						}},
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.5", Public: "10.3", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.6", Public: "10.4", NodeType: spec.NodeType_worker},
						}, IsControl: false,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: make(map[string]string)}},
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.7", Public: "20.3", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.8", Public: "20.4", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}},
				},
			},
		},
		{
			name: "ok-includes-replaced",
			args: args{
				k8sDiffResult: nodePoolDiffResult{
					partialDeletedDynamic: map[string][]string{fmt.Sprintf("pdyn-%s", rnghash): {"2"}},
					partialDeletedStatic:  map[string][]string{fmt.Sprintf("pstat-%s", rnghash): {"2"}},
					deletedDynamic:        map[string][]string{fmt.Sprintf("dyn-%s", rnghash): {"1", "2"}},
					deletedStatic:         map[string][]string{fmt.Sprintf("stat-%s", rnghash): {"1", "2"}},
				},
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					Hash: "hash",
					NodePools: []*spec.NodePool{{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-1",
								},
							},
						}},
						Name: fmt.Sprintf("pdyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.2", Public: "10.2", NodeType: spec.NodeType_apiEndpoint},
							{Name: "2", Private: "1.3", Public: "10.3", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: map[string]string{
							"20.4": "pk20.4",
							"20.5": "pk20.5",
						}}},
						Name: fmt.Sprintf("pstat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.4", Public: "20.4", NodeType: spec.NodeType_master},
							{Name: "2", Private: "1.5", Public: "20.5", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-1",
								},
							},
						}},
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.6", Public: "10.4", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.7", Public: "10.5", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: make(map[string]string)}},
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.8", Public: "20.6", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.9", Public: "20.7", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}},
				}},
				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					Hash: "hash",
					NodePools: []*spec.NodePool{{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-2",
								},
							},
						}},
						Name: fmt.Sprintf("pdyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.2", Public: "10.2", NodeType: spec.NodeType_apiEndpoint},
							{Name: "2", Private: "1.3", Public: "10.3", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: map[string]string{
							"20.4":  "pk20.4",
							"20.10": "pk20.10",
						}}},
						Name: fmt.Sprintf("pstat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.4", Public: "20.4", NodeType: spec.NodeType_master},
							{Name: "2", Private: "1.13", Public: "20.10", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 3,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-3",
								},
							},
						}},
						Name: fmt.Sprintf("new-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.15", Public: "10.21", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.16", Public: "10.22", NodeType: spec.NodeType_worker},
							{Name: "3", Private: "1.17", Public: "10.23", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}},
				}},
			},
			want: &spec.K8Scluster{
				ClusterInfo: &spec.ClusterInfo{
					Name: "current",
					Hash: "hash",
					NodePools: []*spec.NodePool{{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-2",
								},
							},
						}},
						Name: fmt.Sprintf("pdyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.2", Public: "10.2", NodeType: spec.NodeType_apiEndpoint},
							{Name: "2", Private: "1.3", Public: "10.3", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: map[string]string{
							"20.4":  "pk20.4",
							"20.10": "pk20.10",
							"20.5":  "pk20.5",
						}}},
						Name: fmt.Sprintf("pstat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.4", Public: "20.4", NodeType: spec.NodeType_master},
							{Name: fmt.Sprintf("pstat-%s-01", rnghash), Private: "1.13", Public: "20.10", NodeType: spec.NodeType_master},
							{Name: "2", Private: "1.5", Public: "20.5", NodeType: spec.NodeType_master},
						},
						IsControl: true,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 3,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-3",
								},
							},
						}},
						Name: fmt.Sprintf("new-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.15", Public: "10.21", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.16", Public: "10.22", NodeType: spec.NodeType_worker},
							{Name: "3", Private: "1.17", Public: "10.23", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}, {
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							Count: 2,
							Provider: &spec.Provider{
								SpecName:          "test",
								CloudProviderName: "test",
								ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
								Templates: &spec.TemplateRepository{
									Repository: "https://github.com/berops/claudie",
									CommitHash: "hash-1",
								},
							},
						}},
						Name: fmt.Sprintf("dyn-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.6", Public: "10.4", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.7", Public: "10.5", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}, {
						Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{NodeKeys: make(map[string]string)}},
						Name: fmt.Sprintf("stat-%s", rnghash),
						Nodes: []*spec.Node{
							{Name: "1", Private: "1.8", Public: "20.6", NodeType: spec.NodeType_worker},
							{Name: "2", Private: "1.9", Public: "20.7", NodeType: spec.NodeType_worker},
						},
						IsControl: false,
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := craftK8sIR(tt.args.k8sDiffResult, tt.args.current, tt.args.desired)
			equal := proto.Equal(tt.want, got)
			if diff := cmp.Diff(got, tt.want, opts); !equal && diff != "" {
				t.Errorf("craftK8sIR(%v, %v, %v) = %v", tt.args.k8sDiffResult, tt.args.current, tt.args.desired, diff)
			}
		})
	}
}

func Test_k8sAutoscalerDiff(t *testing.T) {
	type args struct {
		current *spec.K8Scluster
		desired *spec.K8Scluster
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "ok-true",
			args: args{
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
					{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							AutoscalerConfig: &spec.AutoscalerConf{Min: 1, Max: 3},
						}},
					},
				}}},
				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
					{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							AutoscalerConfig: &spec.AutoscalerConf{Min: 2, Max: 3},
						}},
					},
				}}},
			},
			want: true,
		},
		{
			name: "ok-false",
			args: args{
				current: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
					{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							AutoscalerConfig: &spec.AutoscalerConf{Min: 1, Max: 3},
						}},
					},
				}}},
				desired: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{NodePools: []*spec.NodePool{
					{
						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
							AutoscalerConfig: &spec.AutoscalerConf{Min: 1, Max: 3},
						}},
					},
				}}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tt.want, k8sAutoscalerDiff(tt.args.current, tt.args.desired), "k8sAutoscalerDiff(%v, %v)", tt.args.current, tt.args.desired)
		})
	}
}

func TestDiff(t *testing.T) {
	rnghash := utils.CreateHash(utils.HashLength)
	current := &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
		Name: "k8s",
		NodePools: []*spec.NodePool{{
			Name:      fmt.Sprintf("np0-%v", rnghash),
			IsControl: true,
			Nodes:     []*spec.Node{{Name: "1", NodeType: spec.NodeType_apiEndpoint}, {Name: "2"}},
			Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
				Count:            2,
				AutoscalerConfig: &spec.AutoscalerConf{Min: 2, Max: 3},
			}},
		}}},
	}
	currentLbs := &spec.LoadBalancers{Clusters: []*spec.LBcluster{
		{
			ClusterInfo: &spec.ClusterInfo{Name: "test1"},
			TargetedK8S: "k8s",
			Roles: []*spec.Role{
				{
					Name:        "api-server",
					Protocol:    "tcp",
					Port:        6443,
					TargetPort:  6443,
					Target:      0,
					TargetPools: []string{"np0"},
					RoleType:    spec.RoleType_ApiServer,
				},
			},
		},
	}}
	type args struct {
		current    *spec.K8Scluster
		desired    *spec.K8Scluster
		currentLbs []*spec.LBcluster
		desiredLbs []*spec.LBcluster
	}
	tests := []struct {
		name string
		args args
		want []*spec.TaskEvent
	}{
		{
			name: "autoscaler-only-change",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig.Min = 3
					return desired
				}(),
			},
			want: []*spec.TaskEvent{{Event: spec.Event_UPDATE, Description: "updating autoscaler config"}},
		},
		{
			name: "autoscaler-disable-different-count",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig = nil
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().Count = 5
					return desired
				}(),
			},
			want: []*spec.TaskEvent{{Event: spec.Event_UPDATE, Description: "adding nodes to k8s cluster"}},
		},
		{
			name: "autoscaler-disable-different-count-2",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig = nil
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().Count = 1
					return desired
				}(),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_DELETE, Description: "deleting nodes from k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "deleting infrastructure of deleted k8s nodes"},
			},
		},
		{
			name: "autoscaler-disable-same-count",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig = nil
					return desired
				}(),
			},
			want: []*spec.TaskEvent{{Event: spec.Event_UPDATE, Description: "updating autoscaler config"}},
		},
		{
			name: "autoscaler-enable-same-count",
			args: args{
				current: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig = nil
					return desired
				}(),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().AutoscalerConfig = &spec.AutoscalerConf{Min: 1, Max: 3}
					return desired
				}(),
			},
			want: []*spec.TaskEvent{{Event: spec.Event_UPDATE, Description: "updating autoscaler config"}},
		},
		{
			name: "delete-only-lb",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: nil,
				desired:    proto.Clone(current).(*spec.K8Scluster),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: "reconciling loadbalancer infrastructure changes"},
				{Event: spec.Event_DELETE, Description: "deleting loadbalancer infrastructure"},
			},
		},
		{
			name: "delete-k8s-nodes",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().Count = 1
					desired.ClusterInfo.NodePools[0].Nodes = desired.ClusterInfo.NodePools[0].Nodes[1:]
					return desired
				}(),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_DELETE, Description: "deleting nodes from k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "deleting infrastructure of deleted k8s nodes"},
			},
		},
		{
			name: "add-k8s-nodes",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().Count = 3
					return desired
				}(),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: "adding nodes to k8s cluster"},
			},
		},
		{
			name: "k8s-add-nodes-and-endpoint-deletion",
			args: args{
				current:    proto.Clone(current).(*spec.K8Scluster),
				currentLbs: nil,
				desiredLbs: nil,
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].Name = "np1"
					return desired
				}(),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: "adding nodes to k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "moving endpoint from old control plane node to a new control plane node"},
				{Event: spec.Event_DELETE, Description: "deleting nodes from k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "deleting infrastructure of deleted k8s nodes"},
			},
		},
		{
			name: "k8s-add-nodes-and-endpoint-deletion",
			args: args{
				current: func() *spec.K8Scluster {
					current := proto.Clone(current).(*spec.K8Scluster)
					current.ClusterInfo.NodePools[0].Nodes[0].NodeType = spec.NodeType_master
					return current
				}(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].Name = fmt.Sprintf("np1-%v", rnghash)
					desired.ClusterInfo.NodePools[0].Nodes[0].NodeType = spec.NodeType_master
					return desired
				}(),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: "adding nodes to k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "loadbalancer target to new control plane nodepool"},
				{Event: spec.Event_DELETE, Description: "deleting nodes from k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "deleting infrastructure of deleted k8s nodes"},
			},
		},
		{
			name: "k8s-deletion-endpoint",
			args: args{
				current: func() *spec.K8Scluster {
					current := proto.Clone(current).(*spec.K8Scluster)
					current.ClusterInfo.NodePools[0].Nodes[1].NodeType = spec.NodeType_apiEndpoint
					current.ClusterInfo.NodePools[0].Nodes[0].NodeType = spec.NodeType_master
					return current
				}(),
				desired: func() *spec.K8Scluster {
					desired := proto.Clone(current).(*spec.K8Scluster)
					desired.ClusterInfo.NodePools[0].GetDynamicNodePool().Count = 1
					return desired
				}(),
				currentLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
				desiredLbs: proto.Clone(currentLbs).(*spec.LoadBalancers).GetClusters(),
			},
			want: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: "moving endpoint from old control plane node to a new control plane node"},
				{Event: spec.Event_DELETE, Description: "deleting nodes from k8s cluster"},
				{Event: spec.Event_UPDATE, Description: "deleting infrastructure of deleted k8s nodes"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Diff(tt.args.current, tt.args.desired, tt.args.currentLbs, tt.args.desiredLbs)
			assert.Equal(t, len(tt.want), len(got))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].Event, got[i].Event)
				assert.Equal(t, tt.want[i].Description, got[i].Description)
			}
		})
	}
}

func Test_k8sNodePoolDiff(t *testing.T) {
	type args struct {
		dynamic        map[string][]string
		static         map[string][]string
		desiredCluster *spec.K8Scluster
	}
	tests := []struct {
		name string
		args args
		want nodePoolDiffResult
	}{
		{
			name: "ok-deleted-static",
			args: args{
				static: map[string][]string{
					"1": {"1", "2", "3"},
					"2": {"1"},
				},
				desiredCluster: &spec.K8Scluster{ClusterInfo: &spec.ClusterInfo{
					Name: "test-01",
					Hash: "rng",
					NodePools: []*spec.NodePool{{
						Type:      &spec.NodePool_StaticNodePool{StaticNodePool: new(spec.StaticNodePool)},
						Name:      "1",
						Nodes:     []*spec.Node{{Name: "1"}, {Name: "3"}},
						IsControl: false,
					}},
				}},
			},
			want: nodePoolDiffResult{
				partialDeletedDynamic: map[string][]string{},
				partialDeletedStatic:  map[string][]string{"1": {"2"}},
				deletedDynamic:        map[string][]string{},
				deletedStatic:         map[string][]string{"2": {"1"}},
				adding:                false,
				deleting:              true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tt.want, k8sNodePoolDiff(tt.args.dynamic, tt.args.static, tt.args.desiredCluster), "k8sNodePoolDiff(%v, %v, %v)", tt.args.dynamic, tt.args.static, tt.args.desiredCluster)
		})
	}
}

func Test_scheduleTasksRetry(t *testing.T) {
	t.Parallel()

	repeat := &spec.Config{
		Version:  0,
		Name:     "test",
		K8SCtx:   &spec.KubernetesContext{},
		Manifest: &spec.Manifest{},
		Clusters: map[string]*spec.ClusterState{
			"test-01": {
				Current: &spec.Clusters{K8S: &spec.K8Scluster{Kubeconfig: "config"}},
				Desired: &spec.Clusters{K8S: &spec.K8Scluster{Kubeconfig: "config"}},
				Events: &spec.Events{
					Events: []*spec.TaskEvent{
						{
							Id:          uuid.NewString(),
							Timestamp:   timestamppb.Now(),
							Event:       spec.Event_UPDATE,
							Task:        &spec.Task{},
							Description: "first task",
							OnError:     &spec.RetryStrategy{Repeat: true},
						},
						{
							Id:          uuid.NewString(),
							Timestamp:   timestamppb.Now(),
							Event:       spec.Event_UPDATE,
							Task:        &spec.Task{},
							Description: "second task",
							OnError:     &spec.RetryStrategy{Repeat: true},
						},
					},
					Ttl:        510,
					Autoscaled: false,
				},
				State: &spec.Workflow{
					Stage:       spec.Workflow_ANSIBLER,
					Status:      spec.Workflow_ERROR,
					Description: "randomly failed",
				},
			},
		},
	}

	cfg, err := store.ConvertFromGRPC(repeat)
	assert.Nil(t, err)

	reschedule, err := scheduleTasks(cfg)
	assert.Nil(t, err)
	assert.True(t, reschedule)

	got, err := store.ConvertToGRPC(cfg)
	assert.Nil(t, err)

	assert.Equal(t, got.Clusters["test-01"].State.Stage, spec.Workflow_NONE)
	assert.Equal(t, got.Clusters["test-01"].State.Status, spec.Workflow_DONE)
	assert.Empty(t, got.Clusters["test-01"].State.Description)
	assert.Empty(t, got.Clusters["test-01"].Events.Ttl)

	repeat.Clusters["test-01"].State.Stage = spec.Workflow_NONE
	repeat.Clusters["test-01"].State.Status = spec.Workflow_DONE
	repeat.Clusters["test-01"].State.Description = ""
	repeat.Clusters["test-01"].Events.Ttl = 0

	for k := range repeat.Clusters {
		for i := range repeat.Clusters[k].Events.Events {
			repeat.Clusters[k].Events.Events[i].Timestamp = nil
			got.Clusters[k].Events.Events[i].Timestamp = nil
		}
	}
	if !proto.Equal(got, repeat) {
		diff := cmp.Diff(got, repeat, opts)
		t.Fatalf("schedule tasks (repeat) failed: %s", diff)
	}
}

func Test_scheduleTasksRollback(t *testing.T) {
	t.Parallel()

	rb := &spec.TaskEvent{
		Id:          uuid.NewString(),
		Timestamp:   timestamppb.Now(),
		Event:       spec.Event_DELETE,
		Task:        &spec.Task{},
		Description: "mid task",
		OnError:     &spec.RetryStrategy{Repeat: true},
	}

	rollback := &spec.Config{
		Version:  0,
		Name:     "test",
		K8SCtx:   &spec.KubernetesContext{},
		Manifest: &spec.Manifest{},
		Clusters: map[string]*spec.ClusterState{
			"test-01": {
				Current: &spec.Clusters{K8S: &spec.K8Scluster{Kubeconfig: "config"}},
				Desired: &spec.Clusters{K8S: &spec.K8Scluster{Kubeconfig: "config"}},
				Events: &spec.Events{
					Events: []*spec.TaskEvent{
						{
							Id:          uuid.NewString(),
							Timestamp:   timestamppb.Now(),
							Event:       spec.Event_UPDATE,
							Task:        &spec.Task{},
							Description: "first task",
							OnError:     &spec.RetryStrategy{Rollback: []*spec.TaskEvent{rb}},
						},
						{
							Id:          uuid.NewString(),
							Timestamp:   timestamppb.Now(),
							Event:       spec.Event_UPDATE,
							Task:        &spec.Task{},
							Description: "second task",
							OnError:     &spec.RetryStrategy{Repeat: true},
						},
					},
					Ttl:        510,
					Autoscaled: false,
				},
				State: &spec.Workflow{
					Stage:       spec.Workflow_ANSIBLER,
					Status:      spec.Workflow_ERROR,
					Description: "randomly failed",
				},
			},
		},
	}

	cfg, err := store.ConvertFromGRPC(rollback)
	assert.Nil(t, err)

	reschedule, err := scheduleTasks(cfg)
	assert.Nil(t, err)
	assert.True(t, reschedule)

	got, err := store.ConvertToGRPC(cfg)
	assert.Nil(t, err)

	assert.Equal(t, got.Clusters["test-01"].State.Stage, spec.Workflow_NONE)
	assert.Equal(t, got.Clusters["test-01"].State.Status, spec.Workflow_DONE)
	assert.Empty(t, got.Clusters["test-01"].State.Description)
	assert.Empty(t, got.Clusters["test-01"].Events.Ttl)

	rollback.Clusters["test-01"].State.Stage = spec.Workflow_NONE
	rollback.Clusters["test-01"].State.Status = spec.Workflow_DONE
	rollback.Clusters["test-01"].State.Description = ""
	rollback.Clusters["test-01"].Events.Ttl = 0

	assert.Equal(t, 1, len(got.Clusters["test-01"].Events.Events))

	// do not compare timestamps.
	got.Clusters["test-01"].Events.Events[0].Timestamp = nil
	rb.Timestamp = nil

	if c := got.Clusters["test-01"].Events.Events[0]; !assert.True(t, proto.Equal(rb, c)) {
		diff := cmp.Diff(rb, c, opts)
		t.Fatalf("schedule tasks (rollback) failed: %s", diff)
	}

	got.Clusters["test-01"].Events.Events = nil
	rollback.Clusters["test-01"].Events.Events = nil

	if !proto.Equal(got, rollback) {
		diff := cmp.Diff(got, rollback, opts)
		t.Fatalf("schedule tasks (rollback) failed: %s", diff)
	}
}
