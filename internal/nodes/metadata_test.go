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

func TestGetAllLabels_SpotDynamicNodePool(t *testing.T) {
	np := dynamicNodePool(true)

	labels, err := GetAllLabels(np, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, ok := labels[string(Spot)]; !ok || got != SpotValue {
		t.Errorf("expected label %q=true, got %q (present=%v)", Spot, got, ok)
	}
}

func TestGetAllLabels_NonSpotDynamicNodePool(t *testing.T) {
	np := dynamicNodePool(false)

	labels, err := GetAllLabels(np, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, ok := labels[string(Spot)]; ok {
		t.Errorf("expected no spot label, but got %q=%q", Spot, got)
	}
}

func TestGetAllLabels_StaticNodePool_NoSpotLabel(t *testing.T) {
	np := staticNodePool()

	labels, err := GetAllLabels(np, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, ok := labels[string(Spot)]; ok {
		t.Errorf("expected no spot label on static nodepool, but got %q=%q", Spot, got)
	}
}

func TestGetAllTaints_SpotDynamicNodePool(t *testing.T) {
	np := dynamicNodePool(true)

	taints := GetAllTaints(np, nil)

	var found bool
	for _, taint := range taints {
		if taint.Key == SpotTaintKey && taint.Value == SpotValue && taint.Effect == k8sV1.TaintEffectNoSchedule {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NoSchedule taint claudie.io/spot=true, got %v", taints)
	}
}

func TestGetAllTaints_NonSpotDynamicNodePool(t *testing.T) {
	np := dynamicNodePool(false)

	taints := GetAllTaints(np, nil)

	for _, taint := range taints {
		if taint.Key == SpotTaintKey {
			t.Errorf("expected no spot taint, but got %v", taint)
		}
	}
}

func TestGetAllTaints_StaticNodePool_NoSpotTaint(t *testing.T) {
	np := staticNodePool()

	taints := GetAllTaints(np, nil)

	for _, taint := range taints {
		if taint.Key == SpotTaintKey {
			t.Errorf("expected no spot taint on static nodepool, but got %v", taint)
		}
	}
}
