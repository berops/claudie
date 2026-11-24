//go:build e2e

package e2e

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestContainerdConfig builds a cluster and checks after each change that containerd of the compute
// nodes is configured with the drop-in from the ConfigMap, and that the control nodes never get it.
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
		// What containerd uses when no drop-in sets image_pull_progress_timeout.
		defaultPullTimeout     = "5m0s"
		containerdDropInFormat = "version = 3\n\n[plugins.'io.containerd.cri.v1.images']\n  image_pull_progress_timeout = '%s'\n"
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)
	e2eClusterKubeClient := newClient(t, e2eClusterKubeconfig(t), kubernetes.NewForConfig)

	testCases := []struct {
		name string
		// Put in the ConfigMap before the apply, "" leaves the ConfigMap as it is.
		configMapPullTimeout string
		manifestFile         string
		wantPullTimeout      string
	}{
		{
			name:                 "create cluster with containerd config",
			configMapPullTimeout: "10m",
			manifestFile:         withContainerdConfigManifestFile,
			wantPullTimeout:      "10m",
		},
		{
			// The InputManifest stays the same, Claudie has to notice the ConfigMap change itself.
			name:                 "change ConfigMap",
			configMapPullTimeout: "15m",
			manifestFile:         withContainerdConfigManifestFile,
			wantPullTimeout:      "15m",
		},
		{
			name:            "remove containerd config",
			manifestFile:    withoutContainerdConfigManifestFile,
			wantPullTimeout: defaultPullTimeout,
		},
		{
			name:            "add containerd config to existing nodes",
			manifestFile:    withContainerdConfigManifestFile,
			wantPullTimeout: "15m",
		},
	}

	for _, step := range testCases {
		if step.configMapPullTimeout != "" {
			applyContainerdConfigMap(t, e2eClusterKubeClient, namespace, configMapName, fmt.Sprintf(containerdDropInFormat, step.configMapPullTimeout))
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
			if got, want := imagePullProgressTimeouts(t, e2eClusterClient, namespace, clusterName, computeNodePool), []string{step.wantPullTimeout, step.wantPullTimeout}; !slices.Equal(got, want) {
				t.Errorf("cluster %s: got image_pull_progress_timeout %v in the containerd config of the %s nodes, want %v", clusterName, got, computeNodePool, want)
			}
			if got, want := imagePullProgressTimeouts(t, e2eClusterClient, namespace, clusterName, controlNodePool), []string{defaultPullTimeout}; !slices.Equal(got, want) {
				t.Errorf("cluster %s: got image_pull_progress_timeout %v in the containerd config of the %s nodes, want the default %v", clusterName, got, controlNodePool, want)
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

// imagePullProgressTimeouts returns image_pull_progress_timeout of containerd on each of the
// cluster's dynamic nodes in nodePool, named as in the InputManifest.
func imagePullProgressTimeouts(t *testing.T, client dynamic.Interface, namespace, cluster, nodePool string) []string {
	t.Helper()

	var timeouts []string
	for _, n := range dynamicNodes(t, client, namespace, cluster) {
		if strings.HasPrefix(n.NodePool, nodePool+"-") {
			timeouts = append(timeouts, imagePullProgressTimeout(t, n))
		}
	}
	return timeouts
}

// imagePullProgressTimeout reads the merged containerd config on the node, as the CRI doesn't
// report the image config of the running containerd.
func imagePullProgressTimeout(t *testing.T, node dynamicNode) string {
	t.Helper()

	const configDump = "containerd config dump"
	var config struct {
		Plugins map[string]struct {
			ImagePullProgressTimeout string `toml:"image_pull_progress_timeout"`
		} `toml:"plugins"`
	}
	if _, err := toml.Decode(runSSH(t, node.IP, dynamicNodeSSHPort, node.Key, configDump), &config); err != nil {
		t.Fatalf("node %s: decoding containerd config dump: %v", node.Name, err)
	}
	return config.Plugins["io.containerd.cri.v1.images"].ImagePullProgressTimeout
}
