package utils

import "strings"

// sanitiseString replaces all white spaces and ":" in the string to "-".
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
