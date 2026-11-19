// Package commoninfra describes how dynamic nodepools of a cluster map onto the
// common networking infrastructure that is shared by all of the nodepools of the
// cluster, i.e. the "common-networking" state that the terraformer renders from
// the "networking" folder of the provider templates.
//
// The package is the single source of truth for this mapping and is used by both
// the terraformer, when rendering the common networking infrastructure, and the
// manager, when deciding whether a change to the nodepools of a cluster requires
// the common networking infrastructure to be reconciled.
//
// The common networking infrastructure is rendered once per [Group]. Within a
// group the templates iterate over the [Region]s used by the nodepools of the group.
// Thus, for additions and deletions of nodepools, the common networking
// infrastructure only changes when the set of (Group, Region) pairs of the cluster
// changes. The remaining inputs of the networking templates (whether the kubernetes
// API port is exposed, and the roles of a loadbalancer) are never changed by
// additions or deletions of nodepools and are reconciled by their own tasks.
package commoninfra

import (
	"cmp"
	"iter"
	"slices"

	"github.com/berops/claudie/internal/extemplates/extofu"
	"github.com/berops/claudie/proto/pb/spec"
)

// Group is a single rendering unit of the common networking infrastructure,
// a provider spec rendered with a single version of its terraformer templates.
type Group struct {
	// SpecName is the name of the provider spec as specified in the InputManifest.
	SpecName string

	// TemplatesPath is the path of the terraformer templates of the provider, see [extofu.TemplatesPath].
	TemplatesPath string

	// Fingerprint is the unique fingerprint of the group, see [extofu.Fingerprint].
	Fingerprint string
}

// Region is a region used within a [Group], along with its external network
// name, as both are used by the networking templates.
type Region = extofu.RegionNetwork

// GroupOf returns the [Group] of the passed in nodepool. For static nodepools,
// which have no common networking infrastructure, false is returned.
func GroupOf(np *spec.NodePool) (Group, bool) {
	d := np.GetDynamicNodePool()
	if d == nil {
		return Group{}, false
	}
	return Group{
		SpecName:      d.GetProvider().GetSpecName(),
		TemplatesPath: extofu.TemplatesPath(d.GetProvider()),
		Fingerprint:   extofu.Fingerprint(d.GetProvider()),
	}, true
}

// RegionOf returns the [Region] of the passed in nodepool. For static nodepools
// false is returned.
func RegionOf(np *spec.NodePool) (Region, bool) {
	d := np.GetDynamicNodePool()
	if d == nil {
		return Region{}, false
	}
	return Region{Region: d.Region, ExternalNetwork: d.ExternalNetworkName}, true
}

// GroupNodePools groups the dynamic nodepools by their [Group]. Static nodepools
// are skipped. The groups are yielded in a deterministic order.
func GroupNodePools(nps []*spec.NodePool) iter.Seq2[Group, []*spec.NodePool] {
	groups := make(map[Group][]*spec.NodePool)
	for _, np := range nps {
		g, ok := GroupOf(np)
		if !ok {
			continue
		}
		groups[g] = append(groups[g], np)
	}

	keys := make([]Group, 0, len(groups))
	for g := range groups {
		keys = append(keys, g)
	}
	slices.SortFunc(keys, func(a, b Group) int {
		return cmp.Or(
			cmp.Compare(a.SpecName, b.SpecName),
			cmp.Compare(a.TemplatesPath, b.TemplatesPath),
		)
	})

	return func(yield func(Group, []*spec.NodePool) bool) {
		for _, g := range keys {
			if !yield(g, groups[g]) {
				return
			}
		}
	}
}

// Regions returns the sorted, unique [Region]s used by the dynamic nodepools
// in the passed in slice. Static nodepools are skipped.
func Regions(nps []*spec.NodePool) []Region {
	var out []Region
	for _, np := range nps {
		r, ok := RegionOf(np)
		if !ok {
			continue
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Region) int {
		return cmp.Or(
			cmp.Compare(a.Region, b.Region),
			cmp.Compare(a.ExternalNetwork, b.ExternalNetwork),
		)
	})
	return slices.Compact(out)
}

// NeedsCommonInfra reports whether adding or removing the nodepool np changes
// the set of (Group, Region) pairs of a cluster with the passed in nodepools,
// in which case the common networking infrastructure of the cluster must be
// reconciled.
//
// This is the case when no other dynamic nodepool in nps, i.e. any nodepool
// with a different name than np, shares the [Group] and [Region] of np. The
// answer is the same for additions and deletions, and it does not matter whether
// nps already contains np or not.
//
// For static nodepools false is always returned.
func NeedsCommonInfra(nps []*spec.NodePool, np *spec.NodePool) bool {
	g, ok := GroupOf(np)
	if !ok {
		return false
	}
	r, _ := RegionOf(np)

	for _, other := range nps {
		if other.Name == np.Name {
			continue
		}
		og, ok := GroupOf(other)
		if !ok || og != g {
			continue
		}
		if or, _ := RegionOf(other); or == r {
			return false
		}
	}
	return true
}
