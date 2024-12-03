package generics

import (
	"cmp"
	"slices"

	"golang.org/x/exp/constraints"
)

type inorder interface {
	constraints.Ordered
	comparable
}

func IterateInOrder[M ~map[K]V, K inorder, V any](m M, f func(k K, v V) error) error {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.SortStableFunc(keys, func(first, second K) int {
		return cmp.Compare(first, second)
	})

	for _, k := range keys {
		if err := f(k, m[k]); err != nil {
			return err
		}
	}

	return nil
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
