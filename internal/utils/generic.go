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
