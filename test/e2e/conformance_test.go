//go:build e2e

package e2e

import (
	"context"
	"testing"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestConformance builds a cluster spanning all provider types and runs the full Cilium
// connectivity test and the Kubernetes conformance suite on it.
func TestConformance(t *testing.T) {
	// Values must match manifests/conformance/cluster.yaml.
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
	if err := runKubernetesConformance(t, childClusterKubeconfig); err != nil {
		t.Fatalf("cluster %s: kubernetes conformance: %v", clusterName, err)
	}
}

// runKubernetesConformance runs the Kubernetes conformance suite with hydrophone. Its
// resources are deleted when the test ends. The logs are kept with go test -artifacts.
func runKubernetesConformance(t *testing.T, kubeconfig string) error {
	t.Helper()

	// Taint keys of the control plane nodes in manifests/conformance/cluster.yaml. The suite treats
	// nodes with any other taint as unschedulable and refuses to start.
	const nonBlockingTaints = "node-role.kubernetes.io/control-plane,test"

	const parallelThreads = "4"

	t.Cleanup(func() {
		if err := runCLI(context.Background(), t, kubeconfig, "hydrophone", "--cleanup"); err != nil {
			t.Logf("cleaning up kubernetes conformance: %v", err)
		}
	})

	return runCLI(t.Context(), t, kubeconfig, "hydrophone", "--conformance",
		"--parallel", parallelThreads,
		"--output-dir", t.ArtifactDir(),
		`--extra-args="--non-blocking-taints=`+nonBlockingTaints+`"`,
	)
}
