//go:build e2e

package e2e

import (
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestK8sUpgrade builds a cluster, bumps its Kubernetes version and checks after each step that
// every node and the control plane run the wanted version and the cluster network works.
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

	testCases := []struct {
		name         string
		manifestFile string
		wantVersion  string
	}{
		{
			name:         "create cluster",
			manifestFile: clusterFromManifestFile,
			wantVersion:  upgradeFromVersion,
		},
		{
			name:         "upgrade cluster",
			manifestFile: clusterToManifestFile,
			wantVersion:  upgradeToVersion,
		},
	}

	for _, step := range testCases {
		manifest := applyInputManifest(t, e2eClusterClient, step.manifestFile, namespace)
		// Gives Claudie time to pick up the change, otherwise the old DONE state passes the wait.
		time.Sleep(time.Minute)
		waitForInputManifest(t, e2eClusterClient, manifest, namespace)

		t.Run(step.name, func(t *testing.T) {
			childClusterKubeconfig := childKubeconfig(t, e2eClusterClient, namespace, clusterName)
			childClusterClient := newClient(t, childClusterKubeconfig, kubernetes.NewForConfig)
			if got := nodeCount(t, childClusterClient); got != expectedNodeCount {
				t.Errorf("cluster %s: got %d nodes, want %d", clusterName, got, expectedNodeCount)
			}
			if got := notReadyNodes(t, childClusterClient); len(got) > 0 {
				t.Errorf("cluster %s: got not Ready nodes %v, want none", clusterName, got)
			}
			if got := nodesNotOnKubeletVersion(t, childClusterClient, step.wantVersion); len(got) > 0 {
				t.Errorf("cluster %s: got nodes on other kubelet versions %v, want all on %s", clusterName, got, step.wantVersion)
			}
			if got, want := controlPlaneVersions(t, childClusterClient), []string{step.wantVersion}; !slices.Equal(got, want) {
				t.Errorf("cluster %s: got control plane versions %v, want %v", clusterName, got, want)
			}
			if err := runCiliumConnectivityTestQuick(t, childClusterKubeconfig); err != nil {
				t.Errorf("cluster %s: cilium connectivity test: %v", clusterName, err)
			}
		})
		// The next step builds on this one, on a broken cluster it would only add noise.
		if t.Failed() {
			t.FailNow()
		}
	}
}

// nodesNotOnKubeletVersion returns the kubelet version of each node of the cluster behind
// client that does not run version, keyed by node name.
func nodesNotOnKubeletVersion(t *testing.T, client kubernetes.Interface, version string) map[string]string {
	t.Helper()

	otherVersions := map[string]string{}
	for _, n := range listNodes(t, client) {
		if v := n.Status.NodeInfo.KubeletVersion; v != version {
			otherVersions[n.Name] = v
		}
	}
	return otherVersions
}

// controlPlaneVersions returns the sorted, distinct image versions of the kube-apiserver,
// kube-controller-manager and kube-scheduler pods of the cluster behind client.
func controlPlaneVersions(t *testing.T, client kubernetes.Interface) []string {
	t.Helper()

	// kubeadm labels its static pods with their component.
	const selector = "component in (kube-apiserver,kube-controller-manager,kube-scheduler)"
	pods, err := retryValue(func() (*corev1.PodList, error) {
		return client.CoreV1().Pods(metav1.NamespaceSystem).List(t.Context(), metav1.ListOptions{LabelSelector: selector})
	})
	if err != nil {
		t.Fatalf("listing control plane pods: %v", err)
	}
	var versions []string
	for _, p := range pods.Items {
		for _, c := range p.Spec.Containers {
			version := c.Image[strings.LastIndex(c.Image, ":")+1:]
			t.Logf("  pod %s: %s", p.Name, version)
			versions = append(versions, version)
		}
	}
	slices.Sort(versions)
	return slices.Compact(versions)
}
