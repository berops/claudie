//Package kubectl provides function for using the kubectl commands
package kubectl

import (
	"fmt"
	"os/exec"
)

// Kubeconfig - the kubeconfig of the cluster as a string
type Kubectl struct {
	Kubeconfig string
	Directory  string
}

// KubectlApply runs kubectl apply in k.Directory directory, with specified manifest and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl apply -f test.yaml -> k.KubectlApply("test.yaml", "")
// example: kubectl apply -f test.yaml -n test -> k.KubectlApply("test.yaml", "test")
func (k *Kubectl) KubectlApply(manifest, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	command := fmt.Sprintf("kubectl apply -f %s %s %s", manifest, kubeconfig, namespace)
	return k.run(command)
}

// KubectlDeleteManifest runs kubectl delete in k.Directory, with specified manifest and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl delete -f test.yaml -> k.KubectlDelete("test.yaml", "")
// example: kubectl delete -f test.yaml -n test -> k.KubectlDelete("test.yaml", "test")
func (k *Kubectl) KubectlDeleteManifest(manifest, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	command := fmt.Sprintf("kubectl delete -f %s %s %s", manifest, kubeconfig, namespace)
	return k.run(command)
}

// KubectlDeleteResource runs kubectl delete in k.Directory, with specified resource, resource name and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl delete ns test -> k.KubectlDeleteResource("ns","test", "")
// example: kubectl delete pod busy-box -n test -> k.KubectlDeleteResource("pod","busy-box", "test")
func (k *Kubectl) KubectlDeleteResource(resource, resourceName, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	command := fmt.Sprintf("kubectl delete %s %s %s %s", resource, resourceName, kubeconfig, namespace)
	return k.run(command)
}

// KubectlDrain runs kubectl drain in k.Directory, on a specified node with flags --ignore-daemonsets --delete-local-data
// example: kubectl drain node1 -> k.KubectlDrain("node1")
func (k *Kubectl) KubectlDrain(nodeName string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	command := fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-local-data %s", nodeName, kubeconfig)
	return k.run(command)
}

// KubectlDescribe runs kubectl describe in k.Directory, on a specified resource, resource name and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl describe pod test -> k.KubectlDescribe("pod","test", "")
// example: kubectl describe pod busy-box -n test -> k.KubectlDescribe("pod","busy-box", "test")
func (k *Kubectl) KubectlDescribe(resource, resourceName, namespace string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	command := fmt.Sprintf("kubectl describe %s %s %s %s", resource, resourceName, kubeconfig, namespace)
	return k.run(command)
}

// KubectlGet runs kubectl get in k.Directory, on a specified resource and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl get ns -> k.KubectlGet("ns", "")
// example: kubectl get pods -n test -> k.KubectlGet("pods", "test")
func (k *Kubectl) KubectlGet(resource, namespace string) ([]byte, error) {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	if namespace != "" {
		namespace = fmt.Sprintf("-n %s", namespace)
	}
	command := fmt.Sprintf("kubectl get %s %s %s", resource, kubeconfig, namespace)
	return k.runWithOutput(command)
}

// KubectlAnnotate runs kubectl annotate in k.Directory, with the specified annotation on a specified resource and resource name
// example: kubectl annotate node node-1 node.longhorn.io/default-node-tags='["zone2"]' -> k.KubectlAnnotate("node","node-1","node.longhorn.io/default-node-tags='["zone2"]")
func (k Kubectl) KubectlAnnotate(resource, resourceName, annotation string) error {
	kubeconfig := fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	command := fmt.Sprintf("kubectl annotate %s %s %s %s", resource, resourceName, annotation, kubeconfig)
	return k.run(command)
}

// run will run the command in a bash shell like "bash -c command"
func (k Kubectl) run(command string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.Directory
	return cmd.Run()
}

// runWithOutput will run the command in a bash shell like "bash -c command" and return the output
func (k Kubectl) runWithOutput(command string) ([]byte, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.Directory
	return cmd.CombinedOutput()
}
