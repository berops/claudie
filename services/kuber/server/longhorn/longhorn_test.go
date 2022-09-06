package longhorn

import (
	"fmt"
	"testing"

	"github.com/Berops/platform/internal/kubectl"
	"github.com/stretchr/testify/require"
)

// NOTE: might need to set kubeconfig and comment out stdout and stderr in runWithOutput()
func TestGetNodeNames(t *testing.T) {
	k := kubectl.Kubectl{Kubeconfig: ""}
	out, err := k.KubectlGetNodeNames()
	require.NoError(t, err)
	fmt.Println(string(out))
}
