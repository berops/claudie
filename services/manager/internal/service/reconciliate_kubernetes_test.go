package service

import (
	"slices"
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
)

func deletionClusters(lbs []*spec.LBcluster, nps ...*spec.NodePool) *spec.Clusters {
	return &spec.Clusters{
		K8S: &spec.K8Scluster{
			ClusterInfo: &spec.ClusterInfo{
				Name:      "test-cluster",
				Hash:      "hash",
				NodePools: nps,
			},
		},
		LoadBalancers: &spec.LoadBalancers{Clusters: lbs},
	}
}

// lbTargeting returns a loadbalancer cluster with a single role targeting the
// given nodepools, which is all that the target pool matching in
// ScheduleDeletionsInNodePools reads.
func lbTargeting(targets ...string) *spec.LBcluster {
	return &spec.LBcluster{
		ClusterInfo: &spec.ClusterInfo{
			Name: "test-lb",
			Hash: "hash",
		},
		Roles: []*spec.Role{
			{
				Name:        "api",
				TargetPools: targets,
			},
		},
	}
}

func namedStaticNodePool(name string) *spec.NodePool {
	np := staticNodePool(nil)
	np.Name = name
	return np
}

func namedDynamicNodePool(name string) *spec.NodePool {
	np := dynamicNodePool(awsProvider("key", "secret"))
	np.Name = name
	return np
}

// expectedStage describes a single pipeline stage of a scheduled deletion,
// exactly one of the subpass slices is set, matching the stage kind.
type expectedStage struct {
	kuber       []spec.StageKuber_SubPassKind
	ansibler    []spec.StageAnsibler_SubPassKind
	terraformer []spec.StageTerraformer_SubPassKind
}

func assertStage(t *testing.T, i int, got *spec.Stage, want expectedStage) {
	t.Helper()

	switch {
	case want.kuber != nil:
		stage := got.GetKuber()
		if stage == nil {
			t.Fatalf("pipeline stage %d = %T, want kuber", i, got.StageKind)
		}
		var kinds []spec.StageKuber_SubPassKind
		for _, sp := range stage.SubPasses {
			kinds = append(kinds, sp.Kind)
		}
		if !slices.Equal(kinds, want.kuber) {
			t.Errorf("pipeline stage %d kuber subpasses = %v, want %v", i, kinds, want.kuber)
		}
	case want.ansibler != nil:
		stage := got.GetAnsibler()
		if stage == nil {
			t.Fatalf("pipeline stage %d = %T, want ansibler", i, got.StageKind)
		}
		var kinds []spec.StageAnsibler_SubPassKind
		for _, sp := range stage.SubPasses {
			kinds = append(kinds, sp.Kind)
		}
		if !slices.Equal(kinds, want.ansibler) {
			t.Errorf("pipeline stage %d ansibler subpasses = %v, want %v", i, kinds, want.ansibler)
		}
	case want.terraformer != nil:
		stage := got.GetTerraformer()
		if stage == nil {
			t.Fatalf("pipeline stage %d = %T, want terraformer", i, got.StageKind)
		}
		var kinds []spec.StageTerraformer_SubPassKind
		for _, sp := range stage.SubPasses {
			kinds = append(kinds, sp.Kind)
		}
		if !slices.Equal(kinds, want.terraformer) {
			t.Errorf("pipeline stage %d terraformer subpasses = %v, want %v", i, kinds, want.terraformer)
		}
	}
}

// The pipeline shape decides which services participate in a node/nodepool
// deletion. For nodepools missing from the current state (drift, e.g. a node
// re-joined the cluster after being removed from the state and the input
// manifest) only the first kuber stage may be scheduled: it deletes the nodes
// from the cluster without committing an update, and since the task delta is
// then never consumed, any update committed by a later stage would be refused
// by the manager, re-running that stage indefinitely.
//
// This holds even when a loadbalancer still lists the absent nodepool among its
// targets: the envoy reconciliation stage commits an update as well, so it has
// to stay out of the pipeline too.
func TestScheduleDeletionsInNodePoolsPipelineShape(t *testing.T) {
	const (
		p1 = "pool-1"
		p2 = "pool-2"
	)

	var (
		kuberDelete = expectedStage{
			kuber: []spec.StageKuber_SubPassKind{spec.StageKuber_DELETE_NODES},
		}
		kuberDeleteWholeNodePool = expectedStage{
			kuber: []spec.StageKuber_SubPassKind{
				spec.StageKuber_DELETE_NODES,
				spec.StageKuber_RECONCILE_LONGHORN_STORAGE_CLASSES,
			},
		}
		proxyPasses = []spec.StageAnsibler_SubPassKind{
			spec.StageAnsibler_INSTALL_VPN,
			spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
			spec.StageAnsibler_COMMIT_PROXY_ENVS,
		}
	)

	tests := []struct {
		name             string
		nodepool         *spec.NodePool
		lbs              []*spec.LBcluster
		diff             NodePoolsDiffResult
		opts             K8sNodeDeletionOptions
		wantPipeline     []expectedStage
		wantWithNodePool bool
		wantNodepool     string
		wantNodes        []string
	}{
		{
			name:     "tracked static nodepool with proxy schedules the full pipeline",
			nodepool: namedStaticNodePool(p1),
			diff:     NodePoolsDiffResult{PartiallyDeleted: NodePoolsViewType{p1: {"node-1"}}},
			opts:     K8sNodeDeletionOptions{IsStatic: true, UseProxy: true},
			wantPipeline: []expectedStage{
				kuberDelete,
				kuberDelete,
				{ansibler: append(
					[]spec.StageAnsibler_SubPassKind{spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES},
					proxyPasses...,
				)},
			},
			wantNodepool: p1,
			wantNodes:    []string{"node-1"},
		},
		{
			name:     "tracked static nodepool without proxy schedules only the utilities cleanup",
			nodepool: namedStaticNodePool(p2),
			diff:     NodePoolsDiffResult{PartiallyDeleted: NodePoolsViewType{p2: {"node-1"}}},
			opts:     K8sNodeDeletionOptions{IsStatic: true},
			wantPipeline: []expectedStage{
				kuberDelete,
				kuberDelete,
				{ansibler: []spec.StageAnsibler_SubPassKind{spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES}},
			},
			wantNodepool: p2,
			wantNodes:    []string{"node-1"},
		},
		{
			name:     "tracked dynamic nodepool with proxy also schedules the terraformer stage",
			nodepool: namedDynamicNodePool(p1),
			diff:     NodePoolsDiffResult{PartiallyDeleted: NodePoolsViewType{p1: {"node-1"}}},
			opts:     K8sNodeDeletionOptions{UseProxy: true},
			wantPipeline: []expectedStage{
				kuberDelete,
				kuberDelete,
				{ansibler: proxyPasses},
				{terraformer: []spec.StageTerraformer_SubPassKind{spec.StageTerraformer_UPDATE_INFRASTRUCTURE}},
			},
			wantNodepool: p1,
			wantNodes:    []string{"node-1"},
		},
		{
			name:         "untracked nodepool schedules only the tracked state removal stage",
			nodepool:     namedStaticNodePool(p1),
			diff:         NodePoolsDiffResult{PartiallyDeleted: NodePoolsViewType{"": {"node-1"}}},
			opts:         K8sNodeDeletionOptions{IsStatic: true, UseProxy: true},
			wantPipeline: []expectedStage{kuberDelete},
			wantNodepool: "",
			wantNodes:    []string{"node-1"},
		},
		{
			name:         "untracked nodepool still referenced by a loadbalancer omits the envoy stage",
			nodepool:     namedStaticNodePool(p1),
			lbs:          []*spec.LBcluster{lbTargeting("ghost")},
			diff:         NodePoolsDiffResult{PartiallyDeleted: NodePoolsViewType{"ghost": {"node-1"}}},
			opts:         K8sNodeDeletionOptions{IsStatic: true, UseProxy: true},
			wantPipeline: []expectedStage{kuberDelete},
			wantNodepool: "ghost",
			wantNodes:    []string{"node-1"},
		},
		{
			name:     "tracked whole nodepool deletion schedules the full pipeline",
			nodepool: namedStaticNodePool(p2),
			diff:     NodePoolsDiffResult{Deleted: NodePoolsViewType{p2: {"node-1", "node-2"}}},
			opts:     K8sNodeDeletionOptions{IsStatic: true, UseProxy: true},
			wantPipeline: []expectedStage{
				kuberDelete,
				kuberDeleteWholeNodePool,
				{ansibler: append(
					[]spec.StageAnsibler_SubPassKind{spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES},
					proxyPasses...,
				)},
			},
			wantWithNodePool: true,
			wantNodepool:     p2,
			wantNodes:        []string{"node-1", "node-2"},
		},
		{
			name:             "untracked whole nodepool deletion schedules only the tracked state removal stage",
			nodepool:         namedStaticNodePool(p2),
			diff:             NodePoolsDiffResult{Deleted: NodePoolsViewType{"ghost": {"node-1"}}},
			opts:             K8sNodeDeletionOptions{IsStatic: true, UseProxy: true},
			wantPipeline:     []expectedStage{kuberDelete},
			wantWithNodePool: true,
			wantNodepool:     "ghost",
			wantNodes:        []string{"node-1"},
		},
		{
			name:             "untracked whole nodepool still referenced by a loadbalancer omits the envoy stage",
			nodepool:         namedStaticNodePool(p2),
			lbs:              []*spec.LBcluster{lbTargeting("ghost")},
			diff:             NodePoolsDiffResult{Deleted: NodePoolsViewType{"ghost": {"node-1"}}},
			opts:             K8sNodeDeletionOptions{IsStatic: true, UseProxy: true},
			wantPipeline:     []expectedStage{kuberDelete},
			wantWithNodePool: true,
			wantNodepool:     "ghost",
			wantNodes:        []string{"node-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := ScheduleDeletionsInNodePools(deletionClusters(tt.lbs, tt.nodepool), &tt.diff, tt.opts)
			if te == nil {
				t.Fatal("ScheduleDeletionsInNodePools() = nil, want a task event")
			}

			if len(te.Pipeline) != len(tt.wantPipeline) {
				t.Fatalf("pipeline has %d stages, want %d", len(te.Pipeline), len(tt.wantPipeline))
			}
			for i, want := range tt.wantPipeline {
				assertStage(t, i, te.Pipeline[i], want)
			}

			update, ok := te.Task.Do.(*spec.Task_Update)
			if !ok {
				t.Fatalf("task action = %T, want update", te.Task.Do)
			}
			delta, ok := update.Update.Delta.(*spec.Update_KDeleteNodes)
			if !ok {
				t.Fatalf("task delta = %T, want KDeleteNodes", update.Update.Delta)
			}

			if delta.KDeleteNodes.WithNodePool != tt.wantWithNodePool {
				t.Errorf("delta WithNodePool = %v, want %v", delta.KDeleteNodes.WithNodePool, tt.wantWithNodePool)
			}
			if delta.KDeleteNodes.Nodepool != tt.wantNodepool {
				t.Errorf("delta Nodepool = %q, want %q", delta.KDeleteNodes.Nodepool, tt.wantNodepool)
			}
			if !slices.Equal(delta.KDeleteNodes.Nodes, tt.wantNodes) {
				t.Errorf("delta Nodes = %v, want %v", delta.KDeleteNodes.Nodes, tt.wantNodes)
			}
		})
	}
}
