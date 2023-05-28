package longhorn

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/berops/claudie/internal/kubectl"
)

// NOTE: might need to set kubeconfig and comment out stdout and stderr in runWithOutput()
func TestGetNodeNames(t *testing.T) {
	k := kubectl.Kubectl{Kubeconfig: ""}
	out, err := k.KubectlGetNodeNames()
	require.NoError(t, err)
	t.Log(string(out))
}