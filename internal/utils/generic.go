package utils

// Contains checks if item is present in the list of items.
func Contains[K comparable](item K, items []K, cmp func(item K, other K) bool) bool {
	for _, v := range items {
		if cmp(item, v) {
			return true
		}
	}

	return false
}
