//go:build e2e

package e2e

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestK8sUpgrade builds a cluster, bumps its Kubernetes version and checks that every node
// runs the new version and the cluster network still works.
func TestK8sUpgrade(t *testing.T) {
	// Values must match manifests/upgrade-cluster/*.yaml.
	const (
		clusterFromManifestFile = "manifests/upgrade-cluster/from-version.yaml"
		clusterToManifestFile   = "manifests/upgrade-cluster/to-version.yaml"
		clusterName             = "upgrade-cluster-test-cluster"
		expectedNodeCount       = 4
		upgradeFromVersion      = "v1.35.0"
		upgradeToVersion        = "v1.36.0"
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)

	manifest := applyInputManifest(t, e2eClusterClient, clusterFromManifestFile, namespace)
	waitForInputManifest(t, e2eClusterClient, manifest, namespace, time.Hour)

	childClusterKubeconfig := childKubeconfig(t, e2eClusterClient, namespace, clusterName)
	childClusterClient := newClient(t, childClusterKubeconfig, kubernetes.NewForConfig)
	if got := readyNodeCount(t, childClusterClient); got != expectedNodeCount {
		t.Fatalf("cluster %s before upgrade: got %d Ready nodes, want %d", clusterName, got, expectedNodeCount)
	}
	if got := nodesNotOnKubeletVersion(t, childClusterClient, upgradeFromVersion); len(got) > 0 {
		t.Fatalf("cluster %s before upgrade: got nodes on other kubelet versions %v, want all on %s", clusterName, got, upgradeFromVersion)
	}

	applyInputManifest(t, e2eClusterClient, clusterToManifestFile, namespace)
	// Gives Claudie time to pick up the change, otherwise the old DONE state passes the wait.
	time.Sleep(time.Minute)
	waitForInputManifest(t, e2eClusterClient, manifest, namespace, time.Hour)

	if got := readyNodeCount(t, childClusterClient); got != expectedNodeCount {
		t.Errorf("cluster %s after upgrade: got %d Ready nodes, want %d", clusterName, got, expectedNodeCount)
	}
	if got := nodesNotOnKubeletVersion(t, childClusterClient, upgradeToVersion); len(got) > 0 {
		t.Errorf("cluster %s after upgrade: got nodes on other kubelet versions %v, want all on %s", clusterName, got, upgradeToVersion)
	}
	if err := runCiliumConnectivityTestQuick(t, childClusterKubeconfig); err != nil {
		t.Errorf("cluster %s after upgrade: cilium connectivity test: %v", clusterName, err)
	}
}

// nodesNotOnKubeletVersion returns the kubelet version of each node of the cluster behind
// client that does not run version, keyed by node name.
func nodesNotOnKubeletVersion(t *testing.T, client kubernetes.Interface, version string) map[string]string {
	t.Helper()

	nodes, err := retryValue(func() (*corev1.NodeList, error) {
		return client.CoreV1().Nodes().List(t.Context(), metav1.ListOptions{})
	})
	if err != nil {
		t.Fatalf("listing nodes: %v", err)
	}
	otherVersions := map[string]string{}
	for _, n := range nodes.Items {
		if v := n.Status.NodeInfo.KubeletVersion; v != version {
			otherVersions[n.Name] = v
		}
	}
	return otherVersions
}
