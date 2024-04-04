package utils

import "strings"

// sanitiseString replaces all white spaces and ":" in the string to "-", and converts everything to lower case.
func SanitiseString(s string) string {
	// convert to lower case
	sanitised := strings.ToLower(s)
	// replace all white space with "-"
	sanitised = strings.ReplaceAll(sanitised, " ", "-")
	// replace all ":" with "-"
	sanitised = strings.ReplaceAll(sanitised, ":", "-")
	// replace all "_" with "-"
	sanitised = strings.ReplaceAll(sanitised, "_", "-")
	return sanitised
}

// watchNamespaceList takes a string input of the form "namespace1,namespace-2,namespace3"
// and splits it into a slice of strings based on the comma separator.
// It returns the slice containing individual namespace strings.
func GetWatchNamespaceList(input string) []string {
	return strings.Split(input, ",")
}