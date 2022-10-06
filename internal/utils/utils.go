package utils

// IsMissing checks if item is missing in the list of items.
func IsMissing[K comparable](item K, items []K) bool {
	for _, v := range items {
		if item == v {
			return false
		}
	}

	return true
}
