package extofu

import (
	"maps"
	"slices"
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
)

func dynamicNodePool(name, specName, commitHash string) *spec.NodePool {
	return &spec.NodePool{
		Name: name,
		Type: &spec.NodePool_DynamicNodePool{
			DynamicNodePool: &spec.DynamicNodePool{
				Provider: &spec.Provider{
					SpecName:          specName,
					CloudProviderName: "gcp",
					Templates: &spec.TemplateRepository{
						Endpoint: &spec.TemplateRepository_Endpoint{
							Url:      "github.com/berops/claudie-config",
							Protocol: spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS,
						},
						CommitHash: commitHash,
						Paths: &spec.TemplateRepository_TemplatePaths{
							Terraformer: "/templates/terraformer",
						},
					},
				},
			},
		},
	}
}

func staticNodePool(name string) *spec.NodePool {
	return &spec.NodePool{
		Name: name,
		Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}},
	}
}

func TestNodePoolsByTemplatesVersion(t *testing.T) {
	names := func(nps []*spec.NodePool) []string {
		out := make([]string, 0, len(nps))
		for _, np := range nps {
			out = append(out, np.Name)
		}
		slices.Sort(out)
		return out
	}

	tests := []struct {
		name string
		nps  []*spec.NodePool
		want map[string][]string // templates path -> sorted nodepool names
	}{
		{
			name: "same provider different commits split into separate groups",
			nps: []*spec.NodePool{
				dynamicNodePool("np1-a1b2c3d", "gcp-1", "1111111111111111111111111111111111111111"),
				dynamicNodePool("np2-a1b2c3d", "gcp-1", "1111111111111111111111111111111111111111"),
				dynamicNodePool("np1-e4f5g6h", "gcp-1", "2222222222222222222222222222222222222222"),
			},
			want: map[string][]string{
				TemplatesPath(dynamicNodePool("", "gcp-1", "1111111111111111111111111111111111111111").GetDynamicNodePool().Provider): {"np1-a1b2c3d", "np2-a1b2c3d"},
				TemplatesPath(dynamicNodePool("", "gcp-1", "2222222222222222222222222222222222222222").GetDynamicNodePool().Provider): {"np1-e4f5g6h"},
			},
		},
		{
			name: "same commit single group",
			nps: []*spec.NodePool{
				dynamicNodePool("np1-a1b2c3d", "gcp-1", "1111111111111111111111111111111111111111"),
				dynamicNodePool("np2-a1b2c3d", "gcp-1", "1111111111111111111111111111111111111111"),
			},
			want: map[string][]string{
				TemplatesPath(dynamicNodePool("", "gcp-1", "1111111111111111111111111111111111111111").GetDynamicNodePool().Provider): {"np1-a1b2c3d", "np2-a1b2c3d"},
			},
		},
		{
			name: "groups by templates version only, not by spec name",
			// The iterator intentionally ignores the SpecName; callers that need
			// per-provider groups must nest it under ByProviderSpecName, as
			// [ClusterBuilder.Init] does.
			nps: []*spec.NodePool{
				dynamicNodePool("np1-a1b2c3d", "gcp-1", "1111111111111111111111111111111111111111"),
				dynamicNodePool("np1-e4f5g6h", "gcp-2", "1111111111111111111111111111111111111111"),
			},
			want: map[string][]string{
				TemplatesPath(dynamicNodePool("", "gcp-1", "1111111111111111111111111111111111111111").GetDynamicNodePool().Provider): {"np1-a1b2c3d", "np1-e4f5g6h"},
			},
		},
		{
			name: "static nodepools are skipped",
			nps: []*spec.NodePool{
				staticNodePool("static-1"),
				dynamicNodePool("np1-a1b2c3d", "gcp-1", "1111111111111111111111111111111111111111"),
				staticNodePool("static-2"),
			},
			want: map[string][]string{
				TemplatesPath(dynamicNodePool("", "gcp-1", "1111111111111111111111111111111111111111").GetDynamicNodePool().Provider): {"np1-a1b2c3d"},
			},
		},
		{
			name: "only static nodepools yield no groups",
			nps:  []*spec.NodePool{staticNodePool("static-1")},
			want: map[string][]string{},
		},
		{
			name: "no nodepools",
			nps:  nil,
			want: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maps.Collect(NodePoolsByTemplatesVersion(tt.nps))

			if len(got) != len(tt.want) {
				t.Fatalf("NodePoolsByTemplatesVersion() = %d groups, want %d", len(got), len(tt.want))
			}
			for path, wantNames := range tt.want {
				group, ok := got[path]
				if !ok {
					t.Errorf("NodePoolsByTemplatesVersion() missing group for path %q", path)
					continue
				}
				if gotNames := names(group); !slices.Equal(gotNames, wantNames) {
					t.Errorf("NodePoolsByTemplatesVersion()[%q] = %v, want %v", path, gotNames, wantNames)
				}
			}
		})
	}
}
