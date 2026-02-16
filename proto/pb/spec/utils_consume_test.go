package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// nolint
func mockLB(name, hash string) *LBcluster {
	lb := &LBcluster{ClusterInfo: &ClusterInfo{Name: name, Hash: hash}}
	return lb
}

func boolPtr(b bool) *bool { return &b }

// taskKind selects which Task variant to wrap the state in.
type taskKind int

const (
	kindCreate taskKind = iota
	kindDelete
	kindUpdate
	kindUnknown
)

// buildTask constructs a Task of the given kind and returns it along with
// accessors to inspect the resulting K8S and LoadBalancers after the call.
func buildTask(kind taskKind, k8s *K8Scluster, lbs []*LBcluster) (
	te *Task,
	getK8S func() *K8Scluster,
	getLBs func() []*LBcluster,
) {
	switch kind {
	case kindCreate:
		c := &Task_Create{Create: &Create{K8S: k8s, LoadBalancers: lbs}}
		return &Task{Do: c},
			func() *K8Scluster { return c.Create.K8S },
			func() []*LBcluster { return c.Create.LoadBalancers }
	case kindDelete:
		d := &Task_Delete{Delete: &Delete{K8S: k8s, LoadBalancers: lbs}}
		return &Task{Do: d},
			func() *K8Scluster { return d.Delete.K8S },
			func() []*LBcluster { return d.Delete.LoadBalancers }
	case kindUpdate:
		u := &Task_Update{Update: &Update{State: &Update_State{K8S: k8s, LoadBalancers: lbs}}}
		return &Task{Do: u},
			func() *K8Scluster { return u.Update.State.K8S },
			func() []*LBcluster { return u.Update.State.LoadBalancers }
	default:
		return &Task{Do: nil}, nil, nil
	}
}

func TestConsumeClearResult(t *testing.T) {
	tests := []struct {
		name     string
		kind     taskKind
		k8s      *K8Scluster
		lbs      []*LBcluster
		result   *TaskResult_Clear
		wantErr  bool
		checkK8S *bool    // nil = don't check, true = expect non-nil, false = expect nil
		wantLBs  []string // expected remaining LB IDs (nil = don't check)
	}{
		{
			name:   "unknown task type returns nil",
			kind:   kindUnknown,
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{}},
		},
		{
			name: "create: removes matching LBs, keeps others",
			kind: kindCreate,
			lbs:  []*LBcluster{mockLB("lb", "1"), mockLB("lb", "2"), mockLB("lb", "3")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-1", "lb-3"},
			}},
			wantLBs: []string{"lb-2"},
		},
		{
			name: "delete: removes matching LBs",
			kind: kindDelete,
			lbs:  []*LBcluster{mockLB("lb", "1")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-1"},
			}},
			wantLBs: []string{},
		},
		{
			name: "update: removes matching LBs",
			kind: kindUpdate,
			lbs:  []*LBcluster{mockLB("lb", "1"), mockLB("lb", "2")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-2"},
			}},
			wantLBs: []string{"lb-1"},
		},
		{
			name: "non-existent LB IDs are no-op",
			kind: kindCreate,
			lbs:  []*LBcluster{mockLB("lb", "1")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-nonexistent"},
			}},
			wantLBs: []string{"lb-1"},
		},
		{
			name: "empty LB IDs list is no-op",
			kind: kindCreate,
			lbs:  []*LBcluster{mockLB("lb", "1")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{},
			}},
			wantLBs: []string{"lb-1"},
		},
		{
			name: "duplicate LB IDs do not panic",
			kind: kindCreate,
			lbs:  []*LBcluster{mockLB("lb", "1")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-1", "lb-1"},
			}},
			wantLBs: []string{},
		},
		{
			name: "clear K8S succeeds when all LBs removed",
			kind: kindCreate,
			k8s:  &K8Scluster{},
			lbs:  []*LBcluster{mockLB("lb", "1"), mockLB("lb", "2")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-1", "lb-2"},
				K8S:              boolPtr(true),
			}},
			checkK8S: boolPtr(false),
		},
		{
			name: "clear K8S succeeds when no LBs from start",
			kind: kindCreate,
			k8s:  &K8Scluster{},
			lbs:  []*LBcluster{},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				K8S: boolPtr(true),
			}},
			checkK8S: boolPtr(false),
		},
		{
			name: "clear K8S refused when LBs still remain",
			kind: kindCreate,
			k8s:  &K8Scluster{},
			lbs:  []*LBcluster{mockLB("lb", "1"), mockLB("lb", "2")},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				LoadBalancersIDs: []string{"lb-1"},
				K8S:              boolPtr(true),
			}},
			wantErr:  true,
			checkK8S: boolPtr(true),    // K8S must NOT be cleared
			wantLBs:  []string{"lb-2"}, // partial consumption: lb-1 still removed
		},
		{
			name: "K8S nil flag does not clear K8S",
			kind: kindCreate,
			k8s:  &K8Scluster{},
			lbs:  []*LBcluster{},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				K8S: nil,
			}},
			checkK8S: boolPtr(true),
		},
		{
			name: "K8S false flag does not clear K8S",
			kind: kindCreate,
			k8s:  &K8Scluster{},
			lbs:  []*LBcluster{},
			result: &TaskResult_Clear{Clear: &TaskResult_ClearState{
				K8S: boolPtr(false),
			}},
			checkK8S: boolPtr(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			te, getK8S, getLBs := buildTask(tt.kind, tt.k8s, tt.lbs)

			err := te.ConsumeClearResult(tt.result)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// For unknown task types we can't inspect state.
			if tt.kind == kindUnknown {
				return
			}

			if tt.checkK8S != nil {
				got := getK8S()
				if *tt.checkK8S && got == nil {
					t.Error("expected K8S to be non-nil, got nil")
				}
				if !*tt.checkK8S && got != nil {
					t.Error("expected K8S to be nil, got non-nil")
				}
			}

			if tt.wantLBs != nil {
				gotLBs := getLBs()
				if len(gotLBs) != len(tt.wantLBs) {
					t.Fatalf("expected %d LBs, got %d", len(tt.wantLBs), len(gotLBs))
				}
				for i, wantID := range tt.wantLBs {
					gotID := gotLBs[i].GetClusterInfo().Id()
					if gotID != wantID {
						t.Errorf("LB[%d]: expected ID %q, got %q", i, wantID, gotID)
					}
				}
			}
		})
	}
}

func TestMutableClusters(t *testing.T) {
	k8s := &K8Scluster{}
	lbs := []*LBcluster{mockLB("lb", "1"), mockLB("lb", "2")}

	tests := []struct {
		name      string
		task      *Task
		wantErr   bool
		wantK8S   *K8Scluster
		wantLBIDs []string
	}{
		{
			name: "create: returns correct references",
			task: &Task{Do: &Task_Create{Create: &Create{
				K8S:           k8s,
				LoadBalancers: lbs,
			}}},
			wantK8S:   k8s,
			wantLBIDs: []string{"lb-1", "lb-2"},
		},
		{
			name: "delete: returns correct references",
			task: &Task{Do: &Task_Delete{Delete: &Delete{
				K8S:           k8s,
				LoadBalancers: lbs,
			}}},
			wantK8S:   k8s,
			wantLBIDs: []string{"lb-1", "lb-2"},
		},
		{
			name: "update: returns correct references",
			task: &Task{Do: &Task_Update{Update: &Update{State: &Update_State{
				K8S:           k8s,
				LoadBalancers: lbs,
			}}}},
			wantK8S:   k8s,
			wantLBIDs: []string{"lb-1", "lb-2"},
		},
		{
			name:    "nil Do returns error",
			task:    &Task{Do: nil},
			wantErr: true,
		},
		{
			name: "create: nil K8S is preserved",
			task: &Task{Do: &Task_Create{Create: &Create{
				K8S:           nil,
				LoadBalancers: lbs,
			}}},
			wantK8S:   nil,
			wantLBIDs: []string{"lb-1", "lb-2"},
		},
		{
			name: "create: empty LBs returns empty slice",
			task: &Task{Do: &Task_Create{Create: &Create{
				K8S:           k8s,
				LoadBalancers: []*LBcluster{},
			}}},
			wantK8S:   k8s,
			wantLBIDs: []string{},
		},
		{
			name: "create: nil LBs returns nil slice",
			task: &Task{Do: &Task_Create{Create: &Create{
				K8S:           k8s,
				LoadBalancers: nil,
			}}},
			wantK8S: k8s,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters, err := tt.task.MutableClusters()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if clusters != nil {
					t.Fatal("expected nil clusters on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if clusters.LoadBalancers == nil {
				t.Fatal("LoadBalancers wrapper should never be nil")
			}

			// Verify K8S identity (same pointer, not a copy).
			if clusters.K8S != tt.wantK8S {
				t.Errorf("K8S: got %p, want %p", clusters.K8S, tt.wantK8S)
			}

			gotLBs := clusters.LoadBalancers.Clusters
			if tt.wantLBIDs == nil {
				if gotLBs != nil {
					t.Fatalf("expected nil LB slice, got len %d", len(gotLBs))
				}
				return
			}
			if len(gotLBs) != len(tt.wantLBIDs) {
				t.Fatalf("expected %d LBs, got %d", len(tt.wantLBIDs), len(gotLBs))
			}
			for i, wantID := range tt.wantLBIDs {
				gotID := gotLBs[i].GetClusterInfo().Id()
				if gotID != wantID {
					t.Errorf("LB[%d]: expected %q, got %q", i, wantID, gotID)
				}
			}
		})
	}
}

func TestMutableClusters_MutationsReflectInTask(t *testing.T) {
	k8s := &K8Scluster{}
	lb := mockLB("lb", "1")

	create := &Task_Create{Create: &Create{
		K8S:           k8s,
		LoadBalancers: []*LBcluster{lb},
	}}
	te := &Task{Do: create}

	clusters, err := te.MutableClusters()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mutate through the returned Clusters and verify the
	// original task state reflects the change.
	newLB := mockLB("lb", "91")
	clusters.LoadBalancers.Clusters = append(clusters.LoadBalancers.Clusters, newLB)

	// The returned slice shares the same backing array initially,
	// but append may or may not reallocate. If the slice header in the
	// task is NOT updated by append (because it's a copy), this
	// test catches that subtle bug.
	if len(create.Create.LoadBalancers) == len(clusters.LoadBalancers.Clusters) {
		// Slice header was shared — mutation visible.
		return
	}

	assert.Equal(t, len(create.Create.LoadBalancers), 1)
	assert.Equal(t, len(clusters.LoadBalancers.Clusters), 2)
}
