//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Values must match manifests/conformance.yaml.
const (
	manifestFile = "manifests/conformance.yaml"
	manifestName = "conformance-test"
	clusterName  = "conformance-test"
	// Taint keys on the control plane nodes. The conformance suite treats nodes with
	// any other taint as unschedulable and refuses to start.
	nonBlockingTaints = "node-role.kubernetes.io/control-plane,test"
)

// TestConformance provisions the multi-cloud cluster from manifests/conformance.yaml
// with Claudie, validates it, and deletes it.
//
// MGMT_KUBECONFIG is the management cluster kubeconfig, CLAUDIE_NAMESPACE the namespace
// Claudie is deployed in. Run with:
//
//	MGMT_KUBECONFIG=/path/to/mgmt.kubeconfig CLAUDIE_NAMESPACE=claudie go test -tags e2e -v -timeout 240m ./test/e2e/ -run TestConformance
func TestConformance(t *testing.T) {
	mgmt := os.Getenv("MGMT_KUBECONFIG")
	if mgmt == "" {
		t.Skip("MGMT_KUBECONFIG must be set")
	}
	namespace := os.Getenv("CLAUDIE_NAMESPACE")
	if namespace == "" {
		t.Fatal("CLAUDIE_NAMESPACE must be set to the namespace Claudie is deployed in")
	}

	if err := ApplyManifest(manifestFile, namespace, mgmt); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Delete the InputManifest first and wait, Claudie still needs the
		// provider secret and templates to destroy the infrastructure.
		if err := DeleteInputManifest(manifestName, namespace, mgmt); err != nil {
			t.Errorf("deleting inputmanifest: %v", err)
			return
		}
		if err := DeleteManifest(manifestFile, namespace, mgmt); err != nil {
			t.Errorf("deleting manifest: %v", err)
		}
	})

	if err := WaitForInputManifestDone(t, manifestName, namespace, mgmt, 60*time.Minute); err != nil {
		t.Fatal(err)
	}

	kubeconfig, err := GetChildClusterKubeconfig(t.Context(), mgmt, namespace, clusterName)
	if err != nil {
		t.Fatal(err)
	}

	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(kubeconfigPath, kubeconfig, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("cilium", func(t *testing.T) {
		t.Cleanup(func() { run(t, kubeconfigPath, "cilium", "connectivity", "test", "--cleanup") })

		if err := run(t, kubeconfigPath, "cilium", "status", "--wait", "--wait-duration", "10m", "--interactive=false"); err != nil {
			t.Fatal(err)
		}
		// Hubble flow validation is off. Cilium runs with monitor aggregation "medium"
		// (services/kuber/manifests/cilium.yaml), which skips most flow checks anyway.
		if err := run(t, kubeconfigPath, "cilium", "connectivity", "test", "--hubble=false", "--timeout", "30m"); err != nil {
			t.Fatal(err)
		}
	})

	// Kubernetes conformance takes about an hour and can't pass on a broken network.
	if t.Failed() {
		return
	}

	t.Run("kubernetes", func(t *testing.T) {
		t.Cleanup(func() { run(t, kubeconfigPath, "hydrophone", "--cleanup") })

		// Parallel to fit into CI, an official conformance submission needs a serial run.
		// The extra args value contains a comma, so it is quoted as a single field.
		if err := run(t, kubeconfigPath, "hydrophone", "--conformance", "--parallel", "8",
			"--output-dir", t.TempDir(),
			`--extra-args="--non-blocking-taints=`+nonBlockingTaints+`"`,
		); err != nil {
			t.Fatal(err)
		}
	})
}

// run runs a CLI against the cluster in kubeconfig and streams its output.
func run(t *testing.T, kubeconfig, name string, args ...string) error {
	t.Logf("running: %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
