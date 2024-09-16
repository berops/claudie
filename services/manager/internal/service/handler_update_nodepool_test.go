package service

import (
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func Test_nodeDiff(t *testing.T) {
	type args struct {
		current *spec.NodePool
		desired *spec.NodePool
	}
	tests := []struct {
		name string
		args args
		want nodeDiffResult
	}{
		{
			name: "ok-reused-and-deleted",
			args: args{
				current: &spec.NodePool{Name: "t0", Nodes: []*spec.Node{
					{Name: "0"}, {Name: "1"},
					{Name: "2"}, {Name: "3"},
					{Name: "4"}, {Name: "5"},
				}, NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{Count: 6}}},
				desired: &spec.NodePool{Name: "t0", Nodes: []*spec.Node{
					{Name: "0"},
					{Name: "3"},
					{Name: "4"}, {Name: "5"}, {Name: "6"},
				}, NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{Count: 4}}},
			},
			want: nodeDiffResult{
				nodepool: "t0",
				deleted:  []*spec.Node{{Name: "1"}, {Name: "2"}},
				reused: []*spec.Node{
					{Name: "0"}, {Name: "3"},
					{Name: "4"}, {Name: "5"},
				},
				added:    []*spec.Node{{Name: "6"}},
				oldCount: 6,
				newCount: 4,
			},
		},
		{
			name: "deleted-endpoint",
			args: args{
				current: &spec.NodePool{Name: "t0", Nodes: []*spec.Node{
					{Name: "0"}, {Name: "1", NodeType: spec.NodeType_apiEndpoint},
					{Name: "2"}, {Name: "3"},
					{Name: "4"}, {Name: "5"},
				}, NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{Count: 6}}},
				desired: &spec.NodePool{Name: "t0", Nodes: []*spec.Node{
					{Name: "0"},
					{Name: "3"},
					{Name: "4"}, {Name: "5"}, {Name: "6"},
				}, NodePoolType: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{Count: 4}}},
			},
			want: nodeDiffResult{
				nodepool: "t0",
				deleted:  []*spec.Node{{Name: "1", NodeType: spec.NodeType_apiEndpoint}, {Name: "2"}},
				reused: []*spec.Node{
					{Name: "0"}, {Name: "3"},
					{Name: "4"}, {Name: "5"},
				},
				endpointDeleted: true,
				added:           []*spec.Node{{Name: "6"}},
				oldCount:        6,
				newCount:        4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, nodeDiff(tt.args.current, tt.args.desired), "nodeDiff(%v, %v)", tt.args.current, tt.args.desired)
		})
	}
}
