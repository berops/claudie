//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestContainerdConfig builds a cluster and checks after each change that the running containerd
// of the compute nodes uses the drop-in from the ConfigMap, and that the control nodes never get it.
func TestContainerdConfig(t *testing.T) {
	const (
		withContainerdConfigManifestFile    = "manifests/containerd-config/with-containerd-config.yaml"
		withoutContainerdConfigManifestFile = "manifests/containerd-config/without-containerd-config.yaml"
		clusterName                         = "containerd-config-cluster"
		// containerdConfigMap of the compute nodepool, the control nodepool has none.
		configMapName     = "compute-containerd"
		controlNodePool   = "htz-ctrl"
		computeNodePool   = "htz-cmpt"
		expectedNodeCount = 3
		// What containerd uses when no drop-in sets max_container_log_line_size.
		defaultLogLineSize = 16384
		// A harmless setting the running containerd reports back.
		containerdDropInFormat = "version = 3\n\n[plugins.'io.containerd.cri.v1.runtime']\n  max_container_log_line_size = %d\n"
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)
	e2eClusterKubeClient := newClient(t, e2eClusterKubeconfig(t), kubernetes.NewForConfig)

	testCases := []struct {
		name string
		// Put in the ConfigMap before the apply, 0 leaves the ConfigMap as it is.
		configMapLogLineSize int
		manifestFile         string
		wantLogLineSize      int
	}{
		{
			name:                 "create cluster with containerd config",
			configMapLogLineSize: 32768,
			manifestFile:         withContainerdConfigManifestFile,
			wantLogLineSize:      32768,
		},
		{
			// The InputManifest stays the same, Claudie has to notice the ConfigMap change itself.
			name:                 "change ConfigMap",
			configMapLogLineSize: 65536,
			manifestFile:         withContainerdConfigManifestFile,
			wantLogLineSize:      65536,
		},
		{
			name:            "remove containerd config",
			manifestFile:    withoutContainerdConfigManifestFile,
			wantLogLineSize: defaultLogLineSize,
		},
		{
			name:            "add containerd config to existing nodes",
			manifestFile:    withContainerdConfigManifestFile,
			wantLogLineSize: 65536,
		},
	}

	for _, step := range testCases {
		if step.configMapLogLineSize != 0 {
			applyContainerdConfigMap(t, e2eClusterKubeClient, namespace, configMapName, fmt.Sprintf(containerdDropInFormat, step.configMapLogLineSize))
		}
		manifest, lastFinished := applyInputManifest(t, e2eClusterClient, step.manifestFile, namespace)
		waitForInputManifest(t, e2eClusterClient, manifest, namespace, lastFinished)

		t.Run(step.name, func(t *testing.T) {
			childClusterClient := newClient(t, childKubeconfig(t, e2eClusterClient, namespace, clusterName), kubernetes.NewForConfig)
			if got := nodeCount(t, childClusterClient); got != expectedNodeCount {
				t.Errorf("cluster %s: got %d nodes, want %d", clusterName, got, expectedNodeCount)
			}
			// A node whose containerd did not come back after the restart is not Ready.
			if got := notReadyNodes(t, childClusterClient); len(got) > 0 {
				t.Errorf("cluster %s: got not Ready nodes %v, want none", clusterName, got)
			}
			if got, want := runningContainerdLogLineSizes(t, e2eClusterClient, namespace, clusterName, computeNodePool), []int{step.wantLogLineSize, step.wantLogLineSize}; !slices.Equal(got, want) {
				t.Errorf("cluster %s: got max_container_log_line_size %v in the running containerd of the %s nodes, want %v", clusterName, got, computeNodePool, want)
			}
			if got, want := runningContainerdLogLineSizes(t, e2eClusterClient, namespace, clusterName, controlNodePool), []int{defaultLogLineSize}; !slices.Equal(got, want) {
				t.Errorf("cluster %s: got max_container_log_line_size %v in the running containerd of the %s nodes, want the default %v", clusterName, got, controlNodePool, want)
			}
		})
		// The next step builds on this one, on a broken cluster it would only add noise.
		if t.Failed() {
			t.FailNow()
		}
	}
}

// applyContainerdConfigMap server-side applies the ConfigMap with config as its only entry.
func applyContainerdConfigMap(t *testing.T, client kubernetes.Interface, namespace, name, config string) {
	t.Helper()

	configMap := applycorev1.ConfigMap(name, namespace).WithData(map[string]string{"config.toml": config})
	err := retryCall(func() error {
		_, err := client.CoreV1().ConfigMaps(namespace).Apply(t.Context(), configMap, metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		return err
	})
	if err != nil {
		t.Fatalf("applying configmap %s/%s: %v", namespace, name, err)
	}
}

// runningContainerdLogLineSizes returns max_container_log_line_size of the running containerd of
// each of the cluster's dynamic nodes in nodePool, named as in the InputManifest.
func runningContainerdLogLineSizes(t *testing.T, client dynamic.Interface, namespace, cluster, nodePool string) []int {
	t.Helper()

	var sizes []int
	for _, n := range dynamicNodes(t, client, namespace, cluster) {
		if strings.HasPrefix(n.NodePool, nodePool+"-") {
			sizes = append(sizes, runningContainerdLogLineSize(t, n))
		}
	}
	return sizes
}

// runningContainerdLogLineSize asks the CRI plugin of the node's containerd for its config, so
// it returns what the daemon loaded, not what is on disk.
func runningContainerdLogLineSize(t *testing.T, node dynamicNode) int {
	t.Helper()

	const criInfo = "crictl --runtime-endpoint unix:///run/containerd/containerd.sock info"
	var info struct {
		Config struct {
			MaxContainerLogLineSize int `json:"maxContainerLogLineSize"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(runSSH(t, node.IP, dynamicNodeSSHPort, node.Key, criInfo)), &info); err != nil {
		t.Fatalf("node %s: decoding crictl info: %v", node.Name, err)
	}
	return info.Config.MaxContainerLogLineSize
}
