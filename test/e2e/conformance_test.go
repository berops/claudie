//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestConformance builds a cluster spanning all provider types and runs the full Cilium
// connectivity test and the Kubernetes conformance suite on it.
func TestConformance(t *testing.T) {
	const (
		clusterManifestFile = "manifests/conformance/cluster.yaml"
		clusterName         = "conformance-test-cluster"
		expectedNodeCount   = 13
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)

	manifest := applyInputManifest(t, e2eClusterClient, clusterManifestFile, namespace)
	waitForInputManifest(t, e2eClusterClient, manifest, namespace)

	childClusterKubeconfig := childKubeconfig(t, e2eClusterClient, namespace, clusterName)
	childClusterClient := newClient(t, childClusterKubeconfig, kubernetes.NewForConfig)
	if got := nodeCount(t, childClusterClient); got != expectedNodeCount {
		t.Fatalf("cluster %s: got %d nodes, want %d", clusterName, got, expectedNodeCount)
	}
	if got := notReadyNodes(t, childClusterClient); len(got) > 0 {
		t.Fatalf("cluster %s: got not Ready nodes %v, want none", clusterName, got)
	}
	if err := runCiliumConnectivityTestFull(t, childClusterKubeconfig); err != nil {
		t.Fatalf("cluster %s: cilium connectivity test: %v", clusterName, err)
	}
	if got := nodesWithFreshMetrics(t, childClusterClient); got != expectedNodeCount {
		t.Fatalf("cluster %s: got fresh metrics-server metrics for %d nodes, want %d", clusterName, got, expectedNodeCount)
	}
	if err := runKubernetesConformance(t, childClusterKubeconfig); err != nil {
		t.Fatalf("cluster %s: kubernetes conformance: %v", clusterName, err)
	}
}

// nodesWithFreshMetrics returns how many nodes have metrics-server metrics under a minute old,
// read through the metrics.k8s.io APIService like kubectl top.
func nodesWithFreshMetrics(t *testing.T, client kubernetes.Interface) int {
	t.Helper()

	raw, err := retryValue(func() ([]byte, error) {
		return client.Discovery().RESTClient().Get().AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").DoRaw(t.Context())
	})
	if err != nil {
		t.Fatalf("listing node metrics: %v", err)
	}
	var metrics struct {
		Items []struct{ Timestamp time.Time }
	}
	if err := json.Unmarshal(raw, &metrics); err != nil {
		t.Fatalf("decoding node metrics: %v", err)
	}

	fresh := 0
	for _, m := range metrics.Items {
		if time.Since(m.Timestamp) < time.Minute {
			fresh++
		}
	}
	return fresh
}

func runKubernetesConformance(t *testing.T, kubeconfig string) error {
	t.Helper()

	const parallelThreads = "4"

	t.Cleanup(func() {
		if err := runCLI(context.Background(), t, kubeconfig, "hydrophone", "--cleanup"); err != nil {
			t.Logf("cleaning up kubernetes conformance: %v", err)
		}
	})

	return runCLI(t.Context(), t, kubeconfig, "hydrophone", "--conformance",
		"--parallel", parallelThreads,
		"--output-dir", t.ArtifactDir(),
	)
}
