package kubectl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//enter the test kubeconfig
const config = ""
const testStr = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-from-other-namespaces
  namespace: claudie
spec:
  ingress:
  - from:
    - podSelector: {}
  podSelector:
    matchLabels: null`

func TestKubectlGet(t *testing.T) {
	Kubectl := Kubectl{Kubeconfig: config}
	_, err := Kubectl.KubectlGet("pods", "kube-system")
	_, err1 := Kubectl.KubectlGet("pods", "")
	require.NoError(t, err, err1)
}

func TestApplyString(t *testing.T) {
	Kubectl := Kubectl{}
	err := Kubectl.KubectlApplyString(testStr, "")
	require.NoError(t, err)
}
