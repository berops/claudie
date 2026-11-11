//go:build e2e

package e2e

import (
	"testing"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestConformance builds a cluster spanning all provider types and runs the full Cilium
// connectivity test and the Kubernetes conformance suite on it.
func TestConformance(t *testing.T) {
	// Values must match manifests/conformance.yaml.
	const (
		clusterManifestFile = "manifests/conformance.yaml"
		clusterName         = "conformance-test-cluster"
		expectedNodeCount   = 13
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)

	manifest := applyInputManifest(t, e2eClusterClient, clusterManifestFile, namespace)
	waitForInputManifest(t, e2eClusterClient, manifest, namespace, time.Hour)

	childClusterKubeconfig := childKubeconfig(t, e2eClusterClient, namespace, clusterName)
	childClusterClient := newClient(t, childClusterKubeconfig, kubernetes.NewForConfig)
	if got := readyNodeCount(t, childClusterClient); got != expectedNodeCount {
		t.Fatalf("cluster %s: got %d Ready nodes, want %d", clusterName, got, expectedNodeCount)
	}

	if err := runCiliumConnectivityTestFull(t, childClusterKubeconfig); err != nil {
		t.Fatalf("cluster %s: cilium connectivity test: %v", clusterName, err)
	}
	if err := runKubernetesConformance(t, childClusterKubeconfig); err != nil {
		t.Fatalf("cluster %s: kubernetes conformance: %v", clusterName, err)
	}
}

// runKubernetesConformance runs the Kubernetes conformance suite with hydrophone. Its
// resources are deleted when the test ends.
func runKubernetesConformance(t *testing.T, kubeconfig string) error {
	t.Helper()

	// Taint keys of the control plane nodes in manifests/conformance.yaml. The suite treats
	// nodes with any other taint as unschedulable and refuses to start.
	const nonBlockingTaints = "node-role.kubernetes.io/control-plane,test"

	t.Cleanup(func() {
		if err := runCLI(t, kubeconfig, "hydrophone", "--cleanup"); err != nil {
			t.Logf("cleaning up kubernetes conformance: %v", err)
		}
	})

	// The extra args value contains a comma, so it is quoted as a single field.
	return runCLI(t, kubeconfig, "hydrophone", "--conformance",
		"--output-dir", t.TempDir(),
		`--extra-args="--non-blocking-taints=`+nonBlockingTaints+`"`,
	)
}
