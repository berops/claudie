package utils

// Contains checks if item is present in the list of items.
func Contains[K any](item K, items []K, cmp func(item K, other K) bool) bool {
	for _, v := range items {
		if cmp(item, v) {
			return true
		}
	}
	return false
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

// MergeMapsFunc traverses maps and passes a common map and key,value pairs to the custom function.
func MergeMapsFunc[M ~map[K]V, K comparable, V any](f func(m map[K]V, k K, v V), maps ...M) M {
	merged := make(M)
	for _, m := range maps {
		for k, v := range m {
			f(merged, k, v)
		}
	}
	return merged
}

// MapValues returns the values of the map in a slice.
func MapValues[M ~map[K]V, K comparable, V any](m M) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}
