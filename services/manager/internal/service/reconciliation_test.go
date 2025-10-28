package service

import (
	"maps"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/berops/claudie/internal/spectesting"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestNodePoolsDiff_Table(t *testing.T) {
	type args struct {
		old NodePoolsViewType
		new NodePoolsViewType
	}

	tests := []struct {
		Name string
		args args
		want NodePoolsDiffResult
	}{
		{
			Name: "ok-deleting-adding-nodes",
			args: args{
				old: NodePoolsViewType{
					"1": []string{"1"},
					"2": []string{"4", "5"},
				},
				new: NodePoolsViewType{
					"1": []string{"1", "6"},
					"2": []string{"5"},
				},
			},
			want: NodePoolsDiffResult{
				PartiallyDeleted: NodePoolsViewType{
					"2": []string{"4"},
				},
				Deleted: NodePoolsViewType{},
				PartiallyAdded: NodePoolsViewType{
					"1": []string{"6"},
				},
				Added: NodePoolsViewType{},
			},
		},
		{
			Name: "ok-adding-only-nodes",
			args: args{
				old: NodePoolsViewType{
					"1": []string{"1"},
					"2": []string{"4", "5"},
				},
				new: NodePoolsViewType{
					"1": []string{"1", "6"},
					"2": []string{"4", "5"},
				},
			},
			want: NodePoolsDiffResult{
				PartiallyDeleted: NodePoolsViewType{},
				Deleted:          NodePoolsViewType{},
				PartiallyAdded: NodePoolsViewType{
					"1": []string{"6"},
				},
				Added: NodePoolsViewType{},
			},
		},
		{
			Name: "ok-adding-nodes-and-nodepools",
			args: args{
				old: NodePoolsViewType{
					"1": []string{"1"},
					"2": []string{"4", "5"},
				},
				new: NodePoolsViewType{
					"1": []string{"1", "6"},
					"2": []string{"4", "5"},
					"3": []string{"9"},
				},
			},
			want: NodePoolsDiffResult{
				PartiallyDeleted: NodePoolsViewType{},
				Deleted:          NodePoolsViewType{},
				PartiallyAdded: NodePoolsViewType{
					"1": []string{"6"},
				},
				Added: NodePoolsViewType{
					"3": []string{"9"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			got := NodePoolsDiff(tt.args.old, tt.args.new)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestNodePoolsDiff_All(t *testing.T) {
	t.Parallel()

	type helperNodePoolsDiffResult struct {
		deletedCounts    map[string][]string
		partiallyAdded   map[string][]string
		partiallyDeleted map[string][]string
	}

	rng := rand.New(rand.NewPCG(uint64(time.Now().Unix()), uint64(time.Now().Unix())+1))

	iter := func(rng *rand.Rand, ci *spec.ClusterInfo) helperNodePoolsDiffResult {
		nodepoolsPreNodeDeletion := helperNodepoolNames(ci)
		toDelete := int(max(10, rng.Uint64()%255))
		_, deletedNodeCounts := spectesting.DeleteNodes(toDelete, ci, spectesting.NodesAll)
		if deletedNodeCounts == nil {
			deletedNodeCounts = make(map[string][]string)
		}
		nodepoolsPostNodeDeletion := helperNodepoolNames(ci)

		// find out if any nodepools were deleted when performing the node deletions.
		var nodepoolsDeleted []string
		for _, np := range nodepoolsPreNodeDeletion {
			if !slices.Contains(nodepoolsPostNodeDeletion, np) {
				nodepoolsDeleted = append(nodepoolsDeleted, np)
			}
		}

		// perform additional nodepool deletions.
		toDelete = int(max(1, rng.Uint64()%5))
		_, deletedNodepoolsCounts := spectesting.DeleteNodePools(toDelete, ci)
		if deletedNodepoolsCounts == nil {
			deletedNodepoolsCounts = make(map[string][]string)
		}

		// fill the information about the nodepool deletions to the just acquired maps.
		for _, ref := range nodepoolsDeleted {
			deletedNodepoolsCounts[ref] = append(deletedNodepoolsCounts[ref], deletedNodeCounts[ref]...)
			delete(deletedNodeCounts, ref)
		}

		// check if any of the node deletions were affected by the nodepools deletions
		for deleted := range deletedNodepoolsCounts {
			for partiallyDeleted := range deletedNodeCounts {
				if deleted == partiallyDeleted {
					deletedNodepoolsCounts[deleted] = append(deletedNodepoolsCounts[deleted], deletedNodeCounts[deleted]...)
					delete(deletedNodeCounts, deleted)
					break
				}
			}
		}

		toAdd := int(max(10, rng.Uint64()%255))
		addedNodesCount := spectesting.AddNodes(toAdd, ci, spectesting.NodesAll)
		if addedNodesCount == nil {
			addedNodesCount = make(map[string][]string)
		}

		return helperNodePoolsDiffResult{
			deletedCounts:    deletedNodepoolsCounts,
			partiallyDeleted: deletedNodeCounts,
			partiallyAdded:   addedNodesCount,
		}
	}

	currentK8sCluster := spectesting.GenerateFakeK8SCluster(true)
	desiredK8sCluster := proto.Clone(currentK8sCluster).(*spec.K8Scluster)

	currentResult := iter(rng, desiredK8sCluster.ClusterInfo)

	currentDynamic, currentStatic := NodePoolsView(currentK8sCluster)
	desiredDynamic, desiredStatic := NodePoolsView(desiredK8sCluster)

	dynamicDiff := NodePoolsDiff(currentDynamic, desiredDynamic)
	staticDiff := NodePoolsDiff(currentStatic, desiredStatic)

	assert.Equal(t, len(currentResult.deletedCounts), len(dynamicDiff.Deleted)+len(staticDiff.Deleted))

	for np, gotNodes := range dynamicDiff.Deleted {
		wantNodes, ok := currentResult.deletedCounts[np]
		assert.Equal(t, len(wantNodes), len(gotNodes))
		assert.True(t, ok)
		for _, node := range gotNodes {
			assert.True(t, slices.Contains(wantNodes, node))
		}
	}

	for np, gotNodes := range staticDiff.Deleted {
		wantNodes, ok := currentResult.deletedCounts[np]
		assert.Equal(t, len(wantNodes), len(gotNodes))
		assert.True(t, ok)
		for _, node := range gotNodes {
			assert.True(t, slices.Contains(wantNodes, node))
		}
	}

	assert.Equal(
		t,
		len(currentResult.partiallyDeleted),
		len(dynamicDiff.PartiallyDeleted)+len(staticDiff.PartiallyDeleted),
	)

	for np, gotNodes := range dynamicDiff.PartiallyDeleted {
		wantNodes, ok := currentResult.partiallyDeleted[np]
		assert.Equal(t, len(wantNodes), len(gotNodes))
		assert.True(t, ok)
		for _, node := range gotNodes {
			assert.True(t, slices.Contains(wantNodes, node))
		}
	}

	for np, gotNodes := range staticDiff.PartiallyDeleted {
		wantNodes, ok := currentResult.partiallyDeleted[np]
		assert.Equal(t, len(wantNodes), len(gotNodes))
		assert.True(t, ok)
		for _, node := range gotNodes {
			assert.True(t, slices.Contains(wantNodes, node))
		}
	}

	assert.Equal(
		t,
		len(currentResult.partiallyAdded),
		len(dynamicDiff.PartiallyAdded)+len(staticDiff.PartiallyAdded),
	)

	for np, gotNodes := range dynamicDiff.PartiallyAdded {
		wantNodes, ok := currentResult.partiallyAdded[np]
		assert.Equal(t, len(wantNodes), len(gotNodes))
		assert.True(t, ok)
		for _, node := range gotNodes {
			assert.True(t, slices.Contains(wantNodes, node))
		}
	}

	for np, gotNodes := range staticDiff.PartiallyAdded {
		wantNodes, ok := currentResult.partiallyAdded[np]
		assert.Equal(t, len(wantNodes), len(gotNodes))
		assert.True(t, ok)
		for _, node := range gotNodes {
			assert.True(t, slices.Contains(wantNodes, node))
		}
	}
}

func TestNodePoolsDiff_PartialAdd_Deletion(t *testing.T) {
	t.Parallel()

	type helperNodePoolsDiffResult struct {
		deletedCounts  map[string][]string
		partiallyAdded map[string][]string
	}

	rng := rand.New(rand.NewPCG(uint64(time.Now().Unix()), uint64(time.Now().Unix())+1))

	var older helperNodePoolsDiffResult
	older.deletedCounts = make(map[string][]string)
	older.partiallyAdded = make(map[string][]string)

	mergeWithOlder := func(current *helperNodePoolsDiffResult) {
		for np := range current.deletedCounts {
			found := false
			for added := range older.partiallyAdded {
				if added == np {
					found = true
					break
				}
			}
			if found {
				// don't include the newly generated nodes in the count...
				current.deletedCounts[np] = slices.DeleteFunc(
					current.deletedCounts[np],
					func(s string) bool { return slices.Contains(older.partiallyAdded[np], s) },
				)
				delete(older.partiallyAdded, np)
			}
		}

		for np, nodes := range older.partiallyAdded {
			current.partiallyAdded[np] = append(current.partiallyAdded[np], nodes...)
		}
		older.partiallyAdded = nil

		maps.Copy(current.deletedCounts, older.deletedCounts)
		older.deletedCounts = nil
	}

	iter := func(rng *rand.Rand, ci *spec.ClusterInfo) helperNodePoolsDiffResult {
		toAdd := int(max(10, rng.Uint64()%255))
		addedNodesCount := spectesting.AddNodes(toAdd, ci, spectesting.NodesAll)
		if addedNodesCount == nil {
			addedNodesCount = make(map[string][]string)
		}

		// perform additional nodepool deletions.
		toDelete := int(max(1, rng.Uint64()%5))
		_, deletedNodepoolsCounts := spectesting.DeleteNodePools(toDelete, ci)
		if deletedNodepoolsCounts == nil {
			deletedNodepoolsCounts = make(map[string][]string)
		}

		// check if any additions were affected.
		for np := range deletedNodepoolsCounts {
			found := false
			for added := range addedNodesCount {
				if added == np {
					found = true
					break
				}
			}
			if found {
				// don't include the newly generated nodes in the count...
				deletedNodepoolsCounts[np] = slices.DeleteFunc(
					deletedNodepoolsCounts[np],
					func(s string) bool { return slices.Contains(addedNodesCount[np], s) },
				)
				delete(addedNodesCount, np)
			}
		}

		return helperNodePoolsDiffResult{
			deletedCounts:  deletedNodepoolsCounts,
			partiallyAdded: addedNodesCount,
		}
	}

	currentK8sCluster := spectesting.GenerateFakeK8SCluster(true)
	desiredK8sCluster := proto.Clone(currentK8sCluster).(*spec.K8Scluster)
	iterations := max(10, rng.Uint64()%100)

	for range iterations {
		currentResult := iter(rng, desiredK8sCluster.ClusterInfo)
		mergeWithOlder(&currentResult)

		currentDynamic, currentStatic := NodePoolsView(currentK8sCluster)
		desiredDynamic, desiredStatic := NodePoolsView(desiredK8sCluster)

		dynamicDiff := NodePoolsDiff(currentDynamic, desiredDynamic)
		staticDiff := NodePoolsDiff(currentStatic, desiredStatic)

		assert.Equal(
			t,
			len(currentResult.partiallyAdded),
			len(dynamicDiff.PartiallyAdded)+len(staticDiff.PartiallyAdded),
		)

		for np, gotNodes := range dynamicDiff.PartiallyAdded {
			wantNodes, ok := currentResult.partiallyAdded[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}

		for np, gotNodes := range staticDiff.PartiallyAdded {
			wantNodes, ok := currentResult.partiallyAdded[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}

		assert.Equal(t, len(currentResult.deletedCounts), len(dynamicDiff.Deleted)+len(staticDiff.Deleted))

		for np, gotNodes := range dynamicDiff.Deleted {
			wantNodes, ok := currentResult.deletedCounts[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}

		for np, gotNodes := range staticDiff.Deleted {
			wantNodes, ok := currentResult.deletedCounts[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}
		older = currentResult
	}
}

func TestNodePoolsDiff_Deletion(t *testing.T) {
	t.Parallel()

	type helperNodePoolsDiffResult struct {
		deletedCounts    map[string][]string
		partiallyDeleted map[string][]string
	}

	var older helperNodePoolsDiffResult
	older.deletedCounts = make(map[string][]string)
	older.partiallyDeleted = make(map[string][]string)

	rng := rand.New(rand.NewPCG(uint64(time.Now().Unix()), uint64(time.Now().Unix())+1))

	mergeWithOlder := func(current *helperNodePoolsDiffResult) {
		for deleted := range current.deletedCounts {
			for partiallyDeleted := range older.partiallyDeleted {
				if deleted == partiallyDeleted {
					current.deletedCounts[deleted] = append(current.deletedCounts[deleted], older.partiallyDeleted[deleted]...)
					delete(older.partiallyDeleted, deleted)
				}
			}
		}

		for deleted, nodes := range older.deletedCounts {
			current.deletedCounts[deleted] = append(current.deletedCounts[deleted], nodes...)
		}
		older.deletedCounts = nil

		for np, nodes := range older.partiallyDeleted {
			current.partiallyDeleted[np] = append(current.partiallyDeleted[np], nodes...)
		}
		older.partiallyDeleted = nil
	}

	iter := func(rng *rand.Rand, ci *spec.ClusterInfo) helperNodePoolsDiffResult {
		nodepoolsPreNodeDeletion := helperNodepoolNames(ci)
		toDelete := int(max(10, rng.Uint64()%255))
		_, deletedNodeCounts := spectesting.DeleteNodes(toDelete, ci, spectesting.NodesAll)
		if deletedNodeCounts == nil {
			deletedNodeCounts = make(map[string][]string)
		}
		nodepoolsPostNodeDeletion := helperNodepoolNames(ci)

		// find out if any nodepools were deleted when performing the node deletions.
		var nodepoolsDeleted []string
		for _, np := range nodepoolsPreNodeDeletion {
			if !slices.Contains(nodepoolsPostNodeDeletion, np) {
				nodepoolsDeleted = append(nodepoolsDeleted, np)
			}
		}

		// perform additional nodepool deletions.
		toDelete = int(max(1, rng.Uint64()%5))
		_, deletedNodepoolsCounts := spectesting.DeleteNodePools(toDelete, ci)
		if deletedNodepoolsCounts == nil {
			deletedNodepoolsCounts = make(map[string][]string)
		}

		for _, ref := range nodepoolsDeleted {
			deletedNodepoolsCounts[ref] = append(deletedNodepoolsCounts[ref], deletedNodeCounts[ref]...)
			delete(deletedNodeCounts, ref)
		}

		// check if any of the node deletions were affected by the nodepools deletions
		for deleted := range deletedNodepoolsCounts {
			for partiallyDeleted := range deletedNodeCounts {
				if deleted == partiallyDeleted {
					deletedNodepoolsCounts[deleted] = append(deletedNodepoolsCounts[deleted], deletedNodeCounts[deleted]...)
					delete(deletedNodeCounts, deleted)
				}
			}
		}
		return helperNodePoolsDiffResult{
			deletedCounts:    deletedNodepoolsCounts,
			partiallyDeleted: deletedNodeCounts,
		}
	}

	currentK8sCluster := spectesting.GenerateFakeK8SCluster(true)
	desiredK8sCluster := proto.Clone(currentK8sCluster).(*spec.K8Scluster)
	iterations := max(10, rng.Uint64()%100)

	for range iterations {
		currentResult := iter(rng, desiredK8sCluster.ClusterInfo)
		mergeWithOlder(&currentResult)

		currentDynamic, currentStatic := NodePoolsView(currentK8sCluster)
		desiredDynamic, desiredStatic := NodePoolsView(desiredK8sCluster)

		dynamicDiff := NodePoolsDiff(currentDynamic, desiredDynamic)
		staticDiff := NodePoolsDiff(currentStatic, desiredStatic)

		assert.Equal(t, len(currentResult.deletedCounts), len(dynamicDiff.Deleted)+len(staticDiff.Deleted))

		for np, gotNodes := range dynamicDiff.Deleted {
			wantNodes, ok := currentResult.deletedCounts[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}

		for np, gotNodes := range staticDiff.Deleted {
			wantNodes, ok := currentResult.deletedCounts[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}

		assert.Equal(
			t,
			len(currentResult.partiallyDeleted),
			len(dynamicDiff.PartiallyDeleted)+len(staticDiff.PartiallyDeleted),
		)

		for np, gotNodes := range dynamicDiff.PartiallyDeleted {
			wantNodes, ok := currentResult.partiallyDeleted[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}

		for np, gotNodes := range staticDiff.PartiallyDeleted {
			wantNodes, ok := currentResult.partiallyDeleted[np]
			assert.Equal(t, len(wantNodes), len(gotNodes))
			assert.True(t, ok)
			for _, node := range gotNodes {
				assert.True(t, slices.Contains(wantNodes, node))
			}
		}
		older = currentResult
	}
}

func helperNodepoolNames(ci *spec.ClusterInfo) []string {
	var result []string
	for _, np := range ci.NodePools {
		result = append(result, np.Name)
	}
	return result
}
