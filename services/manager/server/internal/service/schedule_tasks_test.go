package service

import (
	"fmt"
	"testing"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func Test_findNewAPIEndpointCandidate(t *testing.T) {
	type args struct {
		desired []*spec.NodePool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "find-candidate-ok",
			args: args{
				desired: []*spec.NodePool{
					{Name: "np-0", IsControl: false},
					{Name: "np-1", IsControl: true},
				},
			},
			want: "np-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tt.want, findNewAPIEndpointCandidate(tt.args.desired), "findNewAPIEndpointCandidate(%v)", tt.args.desired)
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
						{Name: fmt.Sprintf("dyn-%s", rnghash), IsControl: true, NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
						{Name: fmt.Sprintf("stat-%s", rnghash), IsControl: true, NodePoolType: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}}},
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
						{Name: fmt.Sprintf("dyn-%s", rnghash), IsControl: true, NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{}}},
						{Name: fmt.Sprintf("stat-%s", rnghash), IsControl: true, NodePoolType: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}}},
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
			assert.Equalf(t, tt.want, endpointNodePoolDeleted(tt.args.k8sDiffResult, tt.args.current), "endpointNodePoolDeleted(%v, %v)", tt.args.k8sDiffResult, tt.args.current)
		})
	}
}
