package kubeEleven

import (
	"fmt"
	"os"
	"strings"
)

// readKubeconfigFromFile reads kubeconfig from a file and returns it as a string
func readKubeconfigFromFile(path string) (string, error) {
	kubeconfigAsByte, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig from file %s : %w", path, err)
	}

	return string(kubeconfigAsByte), nil
}

// sanitiseString replaces all white spaces and ":" in the string to "-".
func sanitiseString(s string) string {
	// convert to lower case
	sanitised := strings.ToLower(s)
	// replace all white space with "-"
	sanitised = strings.ReplaceAll(sanitised, " ", "-")
	// replace all ":" with "-"
	sanitised = strings.ReplaceAll(sanitised, ":", "-")
	return sanitised
}
