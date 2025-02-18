package service

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/spectesting"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func Test_autoscaledEvents(t *testing.T) {
	t.Parallel()

	current := &spec.Clusters{K8S: spectesting.GenerateFakeK8SCluster(false)}
	desired := &spec.Clusters{K8S: proto.Clone(current.K8S).(*spec.K8Scluster)}

	affected, affectedNodes := spectesting.DeleteNodes(5+rand.IntN(50), desired.K8S.ClusterInfo, spectesting.NodesDynamic)
	addedNodes := spectesting.AddNodes(5+rand.IntN(15), desired.K8S.ClusterInfo, spectesting.NodesDynamic)

	for _, dnp := range affected {
		cnp := nodepools.FindByName(dnp.Name, current.K8S.ClusterInfo.NodePools)
		assert.NotNil(t, cnp)
		assert.NotNil(t, cnp.GetDynamicNodePool())
		assert.NotNil(t, dnp.GetDynamicNodePool())

		diff := nodeDiff(dnp.Name, cnp, dnp)
		assert.Equal(t, len(affectedNodes[dnp.Name]), len(diff.deleted), "%#v\n %#v\n%#v\n%#v\n", cnp.GetDynamicNodePool(), dnp.GetDynamicNodePool(), diff, affectedNodes)
		assert.Equal(t, cnp.GetDynamicNodePool().Count, int32(diff.oldCount))
		assert.Equal(t, dnp.GetDynamicNodePool().Count, int32(diff.newCount))
		assert.Equal(t, len(addedNodes[dnp.Name]), len(diff.added))
		assert.Equal(t, int(dnp.GetDynamicNodePool().Count), len(diff.reused)+len(diff.added))

		events := autoscaledEvents(diff, desired)

		var eventCount int
		if len(diff.added) > 0 {
			assert.Equal(t, events[eventCount].Event, spec.Event_UPDATE)
			for _, np := range events[eventCount].Task.UpdateState.K8S.ClusterInfo.NodePools {
				for _, n := range addedNodes[np.Name] {
					assert.True(t, slices.ContainsFunc(np.Nodes, func(node *spec.Node) bool {
						return node.Name == n
					}))
				}
			}
			rollback := events[eventCount].OnError.GetRollback().Tasks[0]
			for np, d := range rollback.Task.DeleteState.K8S.Nodepools {
				for _, n := range d.Nodes {
					assert.True(t, slices.ContainsFunc(addedNodes[np], func(added string) bool {
						return added == n
					}))
				}
			}
			eventCount++
		}
		if len(diff.deleted) > 0 {
			assert.Equal(t, events[eventCount].Event, spec.Event_DELETE)
			for np, d := range events[eventCount].Task.DeleteState.K8S.Nodepools {
				for _, node := range d.Nodes {
					assert.True(t, slices.ContainsFunc(affectedNodes[np], func(n string) bool {
						return node == n
					}))
				}
			}
			eventCount++
		}

		assert.Equal(t, eventCount, len(events))
	}
}

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
			name: "ok-reused--deleted-added",
			args: args{
				current: &spec.NodePool{
					Name: "t0",
					Nodes: []*spec.Node{
						{Name: "0"},
						{Name: "1"},
						{Name: "2"},
						{Name: "3"},
						{Name: "4"},
						{Name: "5"},
					},
					Type: &spec.NodePool_DynamicNodePool{
						DynamicNodePool: &spec.DynamicNodePool{
							Count: 6,
						},
					},
				},
				desired: &spec.NodePool{
					Name: "t0",
					Nodes: []*spec.Node{
						{Name: "0"},
						{Name: "3"},
						{Name: "4"},
						{Name: "5"},
						{Name: "6"},
					},
					Type: &spec.NodePool_DynamicNodePool{
						DynamicNodePool: &spec.DynamicNodePool{
							Count: 6,
						},
					},
				},
			},
			want: nodeDiffResult{
				nodepool: "t0",
				deleted: []*spec.Node{
					{Name: "1"},
					{Name: "2"},
				},
				reused: []*spec.Node{
					{Name: "0"},
					{Name: "3"},
					{Name: "4"},
					{Name: "5"},
				},
				added: []*spec.Node{
					{Name: "6"},
					{Name: "test-01"},
				},
				oldCount: 6,
				newCount: 6,
			},
		},
		{
			name: "ok-reused-and-deleted",
			args: args{
				current: &spec.NodePool{
					Name: "t0",
					Nodes: []*spec.Node{
						{Name: "0"},
						{Name: "1"},
						{Name: "2"},
						{Name: "3"},
						{Name: "4"},
						{Name: "5"},
					},
					Type: &spec.NodePool_DynamicNodePool{
						DynamicNodePool: &spec.DynamicNodePool{
							Count: 6,
						},
					},
				},
				desired: &spec.NodePool{
					Name: "t0",
					Nodes: []*spec.Node{
						{Name: "0"},
						{Name: "3"},
						{Name: "4"},
						{Name: "5"},
						{Name: "6"},
					},
					Type: &spec.NodePool_DynamicNodePool{
						DynamicNodePool: &spec.DynamicNodePool{
							Count: 4,
						},
					},
				},
			},
			want: nodeDiffResult{
				nodepool: "t0",
				deleted: []*spec.Node{
					{Name: "1"},
					{Name: "2"},
				},
				reused: []*spec.Node{
					{Name: "0"},
					{Name: "3"},
					{Name: "4"},
					{Name: "5"},
				},
				added:    nil,
				oldCount: 6,
				newCount: 4,
			},
		},
		{
			name: "deleted-endpoint",
			args: args{
				current: &spec.NodePool{
					Name: "t0",
					Nodes: []*spec.Node{
						{Name: "0"},
						{Name: "1", NodeType: spec.NodeType_apiEndpoint},
						{Name: "2"},
						{Name: "3"},
						{Name: "4"},
						{Name: "5"},
					},
					Type: &spec.NodePool_DynamicNodePool{
						DynamicNodePool: &spec.DynamicNodePool{
							Count: 6,
						},
					},
				},
				desired: &spec.NodePool{
					Name: "t0",
					Nodes: []*spec.Node{
						{Name: "0"},
						{Name: "3"},
						{Name: "4"},
						{Name: "5"},
						{Name: "6"},
					},
					Type: &spec.NodePool_DynamicNodePool{
						DynamicNodePool: &spec.DynamicNodePool{
							Count: 4,
						},
					},
				},
			},
			want: nodeDiffResult{
				nodepool: "t0",
				deleted: []*spec.Node{
					{Name: "1", NodeType: spec.NodeType_apiEndpoint},
					{Name: "2"},
				},
				reused: []*spec.Node{
					{Name: "0"},
					{Name: "3"},
					{Name: "4"},
					{Name: "5"},
				},
				endpointDeleted: true,
				added:           nil,
				oldCount:        6,
				newCount:        4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tt.want, nodeDiff("test", tt.args.current, tt.args.desired), "nodeDiff(%v, %v)", tt.args.current, tt.args.desired)
		})
	}
}
