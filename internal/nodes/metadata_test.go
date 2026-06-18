package nodes

import (
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	k8sV1 "k8s.io/api/core/v1"
)

// dynamicNodePool builds a minimal *spec.NodePool with a DynamicNodePool inside.
func dynamicNodePool(spot bool) *spec.NodePool {
	return &spec.NodePool{
		Name: "test-pool",
		Type: &spec.NodePool_DynamicNodePool{
			DynamicNodePool: &spec.DynamicNodePool{
				Provider: &spec.Provider{
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
func staticNodePool() *spec.NodePool {
	return &spec.NodePool{
		Name: "static-pool",
		Type: &spec.NodePool_StaticNodePool{
			StaticNodePool: &spec.StaticNodePool{},
		},
	}
}

// The spot label and taint are applied together when (and only when) a dynamic
// nodepool has Spot set, so both are exercised by the same set of cases.
var spotMetadataCases = []struct {
	name     string
	np       *spec.NodePool
	wantSpot bool
}{
	{"spot dynamic nodepool", dynamicNodePool(true), true},
	{"non-spot dynamic nodepool", dynamicNodePool(false), false},
	{"static nodepool", staticNodePool(), false},
}

func TestGetAllLabels_Spot(t *testing.T) {
	for _, tc := range spotMetadataCases {
		t.Run(tc.name, func(t *testing.T) {
			labels, err := GetAllLabels(tc.np, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := labels[string(Spot)]
			if tc.wantSpot && (!ok || got != SpotValue) {
				t.Errorf("expected label %q=%q, got %q (present=%v)", Spot, SpotValue, got, ok)
			}
			if !tc.wantSpot && ok {
				t.Errorf("expected no spot label, but got %q=%q", Spot, got)
			}
		})
	}
}

func TestGetAllTaints_Spot(t *testing.T) {
	for _, tc := range spotMetadataCases {
		t.Run(tc.name, func(t *testing.T) {
			var found bool
			for _, taint := range GetAllTaints(tc.np, nil) {
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
