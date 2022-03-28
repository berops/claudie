//Package kubectl provides function for using the kubectl commands
package kubectl

import (
	"fmt"
	"os"
	"os/exec"
)

// Kubeconfig - the kubeconfig of the cluster as a string
type Kubectl struct {
	Kubeconfig string
}

// KubectlApply runs kubectl apply in current directory, with specified manifest and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl apply -f test.yaml -> k.KubectlApply("test.yaml", "")
// example: kubectl apply -f test.yaml -n test -> k.KubectlApply("test.yaml", "test")
func (k *Kubectl) KubectlApply(manifest, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	cmd := exec.Command("kubectl", "apply", "-f", manifest, kubeconfig, namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KubectlDeleteManifest runs kubectl delete in current directory, with specified manifest and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl delete -f test.yaml -> k.KubectlDelete("test.yaml", "")
// example: kubectl delete -f test.yaml -n test -> k.KubectlDelete("test.yaml", "test")
func (k *Kubectl) KubectlDeleteManifest(manifest, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	cmd := exec.Command("kubectl", "delete", "-f", manifest, kubeconfig, namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KubectlDeleteResource runs kubectl delete in current directory, with specified resource, resource name and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl delete ns test -> k.KubectlDeleteResource("ns","test", "")
// example: kubectl delete pod busy-box -n test -> k.KubectlDeleteResource("pod","busy-box", "test")
func (k *Kubectl) KubectlDeleteResource(resource, resourceName, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	cmd := exec.Command("kubectl", "delete", resource, resourceName, kubeconfig, namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KubectlDrain runs kubectl drain in current directory, on a specified node with flags --ignore-daemonsets --delete-local-data
// example: kubectl drain node1 -> k.KubectlDrain("node1")
func (k *Kubectl) KubectlDrain(nodeName string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	cmd := exec.Command("kubectl", "drain", nodeName, "--ignore-daemonsets", "--delete-local-data", kubeconfig)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KubectlDescribe runs kubectl describe in current directory, on a specified resource, resource name and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl describe pod test -> k.KubectlDescribe("pod","test", "")
// example: kubectl describe pod busy-box -n test -> k.KubectlDescribe("pod","busy-box", "test")
func (k *Kubectl) KubectlDescribe(resource, resourceName, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	cmd := exec.Command("kubectl", "describe", resource, resourceName, kubeconfig, namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KubectlGet runs kubectl get in current directory, on a specified resource and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl get ns -> k.KubectlDescribe("ns", "")
// example: kubectl get pods -n test -> k.KubectlDescribe("pods", "test")
func (k *Kubectl) KubectlGet(resource, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	cmd := exec.Command("kubectl", "get", resource, kubeconfig, namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
