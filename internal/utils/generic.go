package utils

import (
	"cmp"
	"golang.org/x/exp/constraints"
	"slices"
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

// Into traverse the elements in k and calls the supplied function f to
// convert them into elements if type V.
func Into[K, V any](k []K, f func(k K) *V) []*V {
	result := make([]*V, 0, len(k))
	for _, k := range k {
		if v := f(k); v != nil {
			result = append(result, v)
		}
	}
	return result
}

func Sum[M ~map[K]V, K comparable, V constraints.Integer | constraints.Float](m M) int {
	var out int
	for _, v := range m {
		out += int(v)
	}
	return out
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
