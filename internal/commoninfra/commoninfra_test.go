package commoninfra

import (
	"reflect"
	"slices"
	"testing"

	"github.com/berops/claudie/internal/extemplates/extofu"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func provider(specName, commit string) *spec.Provider {
	return &spec.Provider{
		SpecName:          specName,
		CloudProviderName: "aws",
		Templates: &spec.TemplateRepository{
			CommitHash: commit,
			Endpoint:   &spec.TemplateRepository_Endpoint{Url: "github.com/berops/claudie-config"},
			Paths:      &spec.TemplateRepository_TemplatePaths{Terraformer: "templates/terraformer"},
		},
	}
}

func dynamic(name string, p *spec.Provider, region, externalNetwork string) *spec.NodePool {
	return &spec.NodePool{
		Name: name,
		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
			Provider:            p,
			Region:              region,
			ExternalNetworkName: externalNetwork,
		}},
	}
}

func static(name string) *spec.NodePool {
	return &spec.NodePool{
		Name: name,
		Type: &spec.NodePool_StaticNodePool{StaticNodePool: &spec.StaticNodePool{}},
	}
}

func TestGroupNodePools(t *testing.T) {
	t.Parallel()

	var (
		aws1v1 = provider("aws-1", "v1")
		aws1v2 = provider("aws-1", "v2")
		aws2v1 = provider("aws-2", "v1")
	)

	nps := []*spec.NodePool{
		dynamic("a", aws2v1, "eu-central-1", ""),
		static("s"),
		dynamic("b", aws1v2, "eu-central-1", ""),
		dynamic("c", aws1v1, "eu-central-1", ""),
		dynamic("d", aws1v1, "us-east-1", ""),
	}

	var got []Group
	members := map[string][]string{}
	for g, group := range GroupNodePools(nps) {
		got = append(got, g)
		for _, np := range group {
			members[g.Fingerprint] = append(members[g.Fingerprint], np.Name)
		}
	}

	g1, _ := GroupOf(nps[3]) // aws-1 v1
	g2, _ := GroupOf(nps[2]) // aws-1 v2
	g3, _ := GroupOf(nps[0]) // aws-2 v1

	// same spec with different templates tags are two groups, different specs are two groups.
	assert.Equal(t, []Group{g1, g2, g3}, got)
	assert.Equal(t, []string{"c", "d"}, members[g1.Fingerprint])
	assert.Equal(t, []string{"b"}, members[g2.Fingerprint])
	assert.Equal(t, []string{"a"}, members[g3.Fingerprint])

	// deterministic regardless of the input order.
	reversed := slices.Clone(nps)
	slices.Reverse(reversed)
	var again []Group
	for g := range GroupNodePools(reversed) {
		again = append(again, g)
	}
	assert.Equal(t, got, again)
}

func TestRegions(t *testing.T) {
	t.Parallel()

	p := provider("os-1", "v1")
	nps := []*spec.NodePool{
		dynamic("a", p, "RegionTwo", "ext"),
		dynamic("b", p, "RegionOne", "ext"),
		dynamic("c", p, "RegionOne", "ext"),
		dynamic("d", p, "RegionOne", "ext-2"),
		static("s"),
	}

	assert.Equal(t, []Region{
		{Region: "RegionOne", ExternalNetwork: "ext"},
		{Region: "RegionOne", ExternalNetwork: "ext-2"},
		{Region: "RegionTwo", ExternalNetwork: "ext"},
	}, Regions(nps))
}

func TestNeedsCommonInfra(t *testing.T) {
	t.Parallel()

	var (
		aws1   = provider("aws-1", "v1")
		aws1v2 = provider("aws-1", "v2")
		aws2   = provider("aws-2", "v1")

		existing = []*spec.NodePool{
			dynamic("a", aws1, "eu-central-1", ""),
			dynamic("b", aws1, "eu-central-1", ""),
			dynamic("c", aws1, "us-east-1", ""),
			dynamic("os", provider("os-1", "v1"), "RegionOne", "ext"),
			static("s"),
		}
	)

	tests := []struct {
		name string
		nps  []*spec.NodePool
		np   *spec.NodePool
		want bool
	}{
		{"add: existing group and region", existing, dynamic("new", aws1, "eu-central-1", ""), false},
		{"add: new region in existing group", existing, dynamic("new", aws1, "eu-west-1", ""), true},
		{"add: new spec", existing, dynamic("new", aws2, "eu-central-1", ""), true},
		{"add: new templates tag", existing, dynamic("new", aws1v2, "eu-central-1", ""), true},
		{"add: same region, different external network", existing, dynamic("new", provider("os-1", "v1"), "RegionOne", "ext-2"), true},
		{"add: static nodepool", existing, static("new"), false},
		{"remove: one of two in group and region", existing, existing[0], false},
		{"remove: last of a region", existing, existing[2], true},
		{"remove: last of a group", existing, existing[3], true},
		{"remove: static nodepool", existing, existing[4], false},
		{"remove: nodepool already removed from the list", existing[1:], existing[0], false},
		{"empty cluster", nil, dynamic("new", aws1, "eu-central-1", ""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NeedsCommonInfra(tt.nps, tt.np))
		})
	}
}

// TestNetworkingInputsAccountedFor guards [NeedsCommonInfra]. Every input of the
// networking templates, i.e. every field of [extofu.Networking], must be listed
// below with how it is accounted for. When a new input is added to the templates
// decide whether it can change by adding or removing a nodepool. If it can, it
// must be included in [NeedsCommonInfra], otherwise the manager would skip the
// reconciliation of the common networking infrastructure for such a change.
func TestNetworkingInputsAccountedFor(t *testing.T) {
	t.Parallel()

	accounted := map[string]string{
		"ClusterData":             "nested, see below",
		"ClusterData.ClusterName": "ignored: fixed for the lifetime of a cluster",
		"ClusterData.ClusterHash": "ignored: fixed for the lifetime of a cluster",
		"ClusterData.ClusterType": "ignored: fixed for the lifetime of a cluster",
		"Provider":                "Group: spec name, templates path, fingerprint",
		"Regions":                 "Region.Region",
		"RegionNetwork":           "Region.Region, Region.ExternalNetwork",
		"K8sData":                 "nested, see below",
		"K8sData.HasAPIServer":    "not changed by nodepool additions/deletions, reconciled by its own task",
		"LBData":                  "nested, see below",
		"LBData.Roles":            "not changed by nodepool additions/deletions, reconciled by its own task",
	}

	var fields []string
	var walk func(prefix string, typ reflect.Type)
	walk = func(prefix string, typ reflect.Type) {
		for f := range typ.Fields() {
			name := prefix + f.Name
			fields = append(fields, name)
			// only descend into the helper structs of the extofu package.
			if f.Type.Kind() == reflect.Struct && f.Type.PkgPath() == reflect.TypeFor[extofu.Networking]().PkgPath() {
				walk(name+".", f.Type)
			}
		}
	}
	walk("", reflect.TypeFor[extofu.Networking]())

	for _, f := range fields {
		if _, ok := accounted[f]; !ok {
			t.Errorf("extofu.Networking.%s is not accounted for by the commoninfra package, "+
				"decide whether it can change by adding or removing a nodepool and update NeedsCommonInfra and this test", f)
		}
	}
	for f := range accounted {
		if !slices.Contains(fields, f) {
			t.Errorf("extofu.Networking.%s no longer exists, remove it from this test", f)
		}
	}
}
