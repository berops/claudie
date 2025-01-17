package generics

import (
	"cmp"
	"iter"
	"maps"
	"slices"

	"golang.org/x/exp/constraints"
)

type inorder interface {
	constraints.Ordered
	comparable
}

func IterateMapInOrder[M ~map[K]V, K inorder, V any](m M) iter.Seq2[K, V] {
	keys := slices.Collect(maps.Keys(m))
	slices.SortStableFunc(keys, func(first, second K) int { return cmp.Compare(first, second) })
	return func(yield func(K, V) bool) {
		for _, k := range keys {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}

// MergeMaps merges two or more maps together, into single map.
func MergeMaps[M ~map[K]V, K comparable, V any](maps ...M) M {
	merged := make(M)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func RemoveDuplicates[K comparable](slice []K) []K {
	keys := make(map[K]bool)
	list := []K{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
