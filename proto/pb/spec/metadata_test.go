package spec

import (
	"testing"

	k8sV1 "k8s.io/api/core/v1"
)

// dynamicNodePool builds a minimal *spec.NodePool with a DynamicNodePool inside.
func dynamicNodePool(spot bool) *NodePool {
	return &NodePool{
		Name: "test-pool",
		Type: &NodePool_DynamicNodePool{
			DynamicNodePool: &DynamicNodePool{
				Provider: &Provider{
					CloudProviderName: "gcp",
					SpecName:          "my-gcp",
				},
				Region: "us-central1",
				Zone:   "us-central1-a",
				Spot:   spot,
			},
		},
	}
}

// staticNodePool builds a minimal *spec.NodePool with a StaticNodePool inside.
func staticNodePool() *NodePool {
	return &NodePool{
		Name: "static-pool",
		Type: &NodePool_StaticNodePool{
			StaticNodePool: &StaticNodePool{},
		},
	}
}

// The spot label and taint are applied together when (and only when) a dynamic
// nodepool has Spot set, so both are exercised by the same set of cases.
var spotMetadataCases = []struct {
	name     string
	np       *NodePool
	wantSpot bool
}{
	{"spot dynamic nodepool", dynamicNodePool(true), true},
	{"non-spot dynamic nodepool", dynamicNodePool(false), false},
	{"static nodepool", staticNodePool(), false},
}

func TestGetAllLabels_Spot(t *testing.T) {
	for _, tc := range spotMetadataCases {
		t.Run(tc.name, func(t *testing.T) {
			labels, err := tc.np.AllLabels(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := labels[string(SpotKey)]
			if tc.wantSpot && (!ok || got != SpotValue) {
				t.Errorf("expected label %q=%q, got %q (present=%v)", SpotKey, SpotValue, got, ok)
			}
			if !tc.wantSpot && ok {
				t.Errorf("expected no spot label, but got %q=%q", SpotKey, got)
			}
		})
	}
}

func TestGetAllTaints_Spot(t *testing.T) {
	for _, tc := range spotMetadataCases {
		t.Run(tc.name, func(t *testing.T) {
			var found bool
			for _, taint := range tc.np.AllTaints(nil) {
				if taint.Key == SpotTaintKey {
					if taint.Value != SpotValue || taint.Effect != k8sV1.TaintEffectNoSchedule {
						t.Errorf("unexpected spot taint shape: %v", taint)
					}
					found = true
				}
			}
			if found != tc.wantSpot {
				t.Errorf("spot taint present=%v, want %v", found, tc.wantSpot)
			}
		})
	}
}
