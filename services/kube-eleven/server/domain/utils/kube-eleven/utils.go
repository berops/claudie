package kube_eleven

import (
	"fmt"
	"os"
)

// readKubeconfigFromFile reads kubeconfig from a file and returns it as a string
func readKubeconfigFromFile(path string) (string, error) {
	kubeconfigAsByte, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig from file %s : %w", path, err)
	}

	return string(kubeconfigAsByte), nil
}
