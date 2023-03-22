// Package kubectl provides function for using the kubectl commands
package kubectl

import (
	"fmt"
	"os/exec"
	"strings"

	comm "github.com/berops/claudie/internal/command"
)

// Kubeconfig - the kubeconfig of the cluster as a string
// when left empty, kuber uses default kubeconfig,
// MaxKubectlRetries when unset/set=0 will use defaultMaxKubectlRetries = 10
type Kubectl struct {
	Kubeconfig        string
	Directory         string
	MaxKubectlRetries int
}

const (
	defaultMaxKubectlRetries = 10
	getEtcdPodsCmd           = "get pods -n kube-system --no-headers -o custom-columns=\":metadata.name\" | grep etcd"
	exportEtcdEnvsCmd        = `export ETCDCTL_API=3 && 
		export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt && 
		export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/healthcheck-client.crt && 
		export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/healthcheck-client.key`
	kubectlTimeout = 3 * 60 // cancel kubectl command after kubectlTimeout seconds
)

// KubectlApply runs kubectl apply in k.Directory directory, with specified manifest
// example: kubectl apply -f test.yaml -> k.KubectlApply("test.yaml")
// example: kubectl apply -f test.yaml -n test -> k.KubectlApply("test.yaml", "-n", "test")
func (k *Kubectl) KubectlApply(manifest string, options ...string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl apply -f %s %s", manifest, kubeconfig)
	return k.run(command, options...)
}

// KubectlApplyString runs kubectl apply in k.Directory directory, with specified string data
// example: echo 'Kind: Pod ...' | kubectl apply -f - -> k.KubectlApply("Kind: Pod ...")
func (k *Kubectl) KubectlApplyString(str string, options ...string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("echo '%s' | kubectl apply -f - %s", str, kubeconfig)
	return k.run(command, options...)
}

// KubectlDeleteManifest runs kubectl delete in k.Directory, with specified manifest
// example: kubectl delete -f test.yaml -> k.KubectlDelete("test.yaml")
// example: kubectl delete -f test.yaml -n test -> k.KubectlDelete("test.yaml", "-n", "test")
func (k *Kubectl) KubectlDeleteManifest(manifest string, options ...string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl delete -f %s %s", manifest, kubeconfig)
	return k.run(command, options...)
}

// KubectlDeleteResource runs kubectl delete in k.Directory, with specified resource, resource name
// example: kubectl delete ns test -> k.KubectlDeleteResource("ns","test")
// example: kubectl delete pod busy-box -n test -> k.KubectlDeleteResource("pod","busy-box", "-n","test")
func (k *Kubectl) KubectlDeleteResource(resource, resourceName string, options ...string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl delete %s %s %s", resource, resourceName, kubeconfig)
	return k.run(command, options...)
}

// KubectlDrain runs kubectl drain in k.Directory, on a specified node with flags --ignore-daemonsets --delete-emptydir-data
// example: kubectl drain node1 -> k.KubectlDrain("node1")
func (k *Kubectl) KubectlDrain(nodeName string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-emptydir-data %s", nodeName, kubeconfig)
	return k.run(command)
}

// KubectlDescribe runs kubectl describe in k.Directory, on a specified resource, resource name and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl describe pod test -> k.KubectlDescribe("pod","test")
// example: kubectl describe pod busy-box -n test -> k.KubectlDescribe("pod","busy-box", "-n", "test")
func (k *Kubectl) KubectlDescribe(resource, resourceName string, options ...string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl describe %s %s %s", resource, resourceName, kubeconfig)
	return k.run(command, options...)
}

// KubectlGet runs kubectl get in k.Directory, on a specified resource and specified namespace
// if namespace is empty string, the kubectl apply will not use -n flag
// example: kubectl get ns -> k.KubectlGet("ns")
// example: kubectl get pods -n test -> k.KubectlGet("pods","-n", "test")
func (k *Kubectl) KubectlGet(resource string, options ...string) ([]byte, error) {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl get %s %s", resource, kubeconfig)
	return k.runWithOutput(command, options...)
}

// KubectlAnnotate runs kubectl annotate in k.Directory, with the specified annotation on a specified resource and resource name
// example: kubectl annotate node node-1 node.longhorn.io/default-node-tags='["zone2"]' -> k.KubectlAnnotate("node","node-1","node.longhorn.io/default-node-tags='["zone2"]")
func (k *Kubectl) KubectlAnnotate(resource, resourceName, annotation string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl annotate %s %s %s %s", resource, resourceName, annotation, kubeconfig)
	return k.run(command)
}

// KubectlLabel runs kubectl label in k.Directory, with the specified label on a specified resource and resource name
// example: kubectl label node node-1 label=value -> k.KubectlLabel("node","node-1","label=value")
func (k *Kubectl) KubectlLabel(resource, resourceName, label string, options ...string) error {
	kubeconfig := k.getKubeconfig()
	command := fmt.Sprintf("kubectl label %s %s %s %s", resource, resourceName, label, kubeconfig)
	return k.run(command, options...)
}

// KubectlGetNodeNames will find a node names for a particular cluster
// return slice of node names and nil if successful, nil and error otherwise
func (k *Kubectl) KubectlGetNodeNames() ([]byte, error) {
	kubeconfig := k.getKubeconfig()
	nodesQueryCmd := fmt.Sprintf("kubectl get nodes -n kube-system --no-headers -o custom-columns=\":metadata.name\" %s", kubeconfig)
	return k.runWithOutput(nodesQueryCmd)
}

// getEtcdPods finds all etcd pods in cluster
// returns slice of pod names and nil if successful, nil and error otherwise
func (k *Kubectl) KubectlGetEtcdPods(masterNodeName string) ([]byte, error) {
	kubeconfig := k.getKubeconfig()
	// get etcd pods name
	podsQueryCmd := fmt.Sprintf("kubectl %s %s-%s", kubeconfig, getEtcdPodsCmd, masterNodeName)
	return k.runWithOutput(podsQueryCmd)
}

func (k *Kubectl) KubectlExecEtcd(etcdPod, etcdctlCmd string) ([]byte, error) {
	kubeconfig := k.getKubeconfig()
	kcExecEtcdCmd := fmt.Sprintf("kubectl %s -n kube-system exec -i %s -- /bin/sh -c \" %s && %s \"",
		kubeconfig, etcdPod, exportEtcdEnvsCmd, etcdctlCmd)
	return k.runWithOutput(kcExecEtcdCmd)
}

func (k *Kubectl) KubectlPatch(resource, resourceName, patchPath string, options ...string) error {
	command := fmt.Sprintf("kubectl patch %s %s -p '%s' %s", resource, resourceName, patchPath, k.getKubeconfig())
	return k.run(command, options...)
}

// run will run the command in a bash shell like "bash -c command options".
func (k Kubectl) run(command string, options ...string) error {
	cmd := exec.Command("bash", "-c", strings.Join(append([]string{command}, options...), " "))
	cmd.Dir = k.Directory
	if err := cmd.Run(); err != nil {
		retryCount := k.MaxKubectlRetries
		if k.MaxKubectlRetries == 0 {
			retryCount = defaultMaxKubectlRetries
		}
		retryCmd := comm.Cmd{Command: command, Options: options, Dir: k.Directory, CommandTimeout: kubectlTimeout}
		if err = retryCmd.RetryCommand(retryCount); err != nil {
			return err
		}
	}
	return nil
}

// runWithOutput will run the command in a bash shell like "bash -c command options" and return the output
func (k Kubectl) runWithOutput(command string, options ...string) ([]byte, error) {
	var result []byte
	var err error
	cmd := exec.Command("bash", "-c", strings.Join(append([]string{command}, options...), " "))
	cmd.Dir = k.Directory
	result, err = cmd.CombinedOutput()
	if err != nil {
		retryCount := k.MaxKubectlRetries
		if k.MaxKubectlRetries == 0 {
			retryCount = defaultMaxKubectlRetries
		}
		cmd := comm.Cmd{Command: command, Options: options, Dir: k.Directory, CommandTimeout: kubectlTimeout}
		result, err = cmd.RetryCommandWithOutput(retryCount)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// getKubeconfig function returns either the "--kubeconfig <(echo ...)" if kubeconfig is specified, or empty string of none is given
func (k Kubectl) getKubeconfig() string {
	if k.Kubeconfig == "" {
		return ""
	} else {
		return fmt.Sprintf("--kubeconfig <(echo '%s')", k.Kubeconfig)
	}
}
