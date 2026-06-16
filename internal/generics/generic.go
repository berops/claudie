package generics

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

type inorder interface {
	cmp.Ordered
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
