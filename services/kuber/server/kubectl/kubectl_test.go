package kubectl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//enter the test kubeconfig
const config = ""

func TestKubectlGet(t *testing.T) {
	Kubectl := Kubectl{Kubeconfig: config}
	_, err := Kubectl.KubectlGet("pods", "kube-system")
	_, err1 := Kubectl.KubectlGet("pods", "")
	require.NoError(t, err, err1)
}
