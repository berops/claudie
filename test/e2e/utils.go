//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/berops/claudie/api/crd/inputmanifest/v1beta1"
	"github.com/berops/claudie/internal/kubectl"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetChildClusterKubeconfig returns the kubeconfig of a cluster provisioned by Claudie.
//
// Claudie stores it in the management cluster as a Secret in the namespace Claudie
// runs in, labeled claudie.io/output=kubeconfig and claudie.io/cluster=<clusterName>,
// under the "kubeconfig" key.
//
// managementKubeconfig is the path to the management cluster kubeconfig.
// namespace is the namespace Claudie runs in; empty searches all namespaces.
func GetChildClusterKubeconfig(ctx context.Context, managementKubeconfig, namespace, clusterName string) ([]byte, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", managementKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("loading management kubeconfig %q: %w", managementKubeconfig, err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating management cluster client: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		"claudie.io/output":  "kubeconfig",
		"claudie.io/cluster": clusterName,
	}).String()

	secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("listing kubeconfig secrets for cluster %q: %w", clusterName, err)
	}

	switch len(secrets.Items) {
	case 0:
		return nil, fmt.Errorf("no kubeconfig secret found for cluster %q (selector %q, namespace %q)", clusterName, selector, namespace)
	case 1:
	default:
		return nil, fmt.Errorf("found %d kubeconfig secrets for cluster %q, set the namespace to narrow it down", len(secrets.Items), clusterName)
	}

	secret := secrets.Items[0]
	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok || len(kubeconfig) == 0 {
		return nil, fmt.Errorf("secret %s/%s has no kubeconfig key", secret.Namespace, secret.Name)
	}

	// client-go already base64-decodes Secret data, this is the raw kubeconfig.
	return kubeconfig, nil
}

// ApplyManifest runs `kubectl apply -f path` in namespace against the management cluster.
func ApplyManifest(path, namespace, mgmtKubeconfig string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3, Stdout: os.Stdout, Stderr: os.Stderr}
	return kc.KubectlApply(path, "-n", namespace, "--kubeconfig", mgmtKubeconfig)
}

// DeleteManifest runs `kubectl delete -f path` in namespace against the management cluster.
func DeleteManifest(path, namespace, mgmtKubeconfig string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3, Stdout: os.Stdout, Stderr: os.Stderr}
	return kc.KubectlDeleteManifest(path, "-n", namespace, "--ignore-not-found", "--kubeconfig", mgmtKubeconfig)
}

// DeleteInputManifest deletes the InputManifest and waits until Claudie has
// destroyed the infrastructure and removed its finalizer.
func DeleteInputManifest(name, namespace, mgmtKubeconfig string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3, Stdout: os.Stdout, Stderr: os.Stderr}
	return kc.KubectlDeleteResource("inputmanifest", name, "-n", namespace, "--ignore-not-found", "--timeout=60m", "--kubeconfig", mgmtKubeconfig)
}

// WaitForInputManifestDone polls the InputManifest status until it is stably done,
// stably in error, or the timeout expires.
func WaitForInputManifestDone(t *testing.T, name, namespace, mgmtKubeconfig string, timeout time.Duration) error {
	t.Helper()

	const (
		interval = 15 * time.Second
		// consecutive polls needed to trust the state, the status can flip during reconciliation.
		needed = 8
	)

	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var done, failed int
	for {
		select {
		case <-t.Context().Done():
			return t.Context().Err()
		case <-deadline:
			return fmt.Errorf("inputmanifest %s/%s not done after %s", namespace, name, timeout)
		case <-ticker.C:
		}

		out, err := kc.KubectlGet("inputmanifest", name, "-n", namespace, "-o", "json", "--kubeconfig", mgmtKubeconfig)
		if err != nil {
			t.Logf("failed to get inputmanifest %s/%s: %v", namespace, name, err)
			continue
		}

		var im v1beta1.InputManifest
		if err := json.Unmarshal(out, &im); err != nil {
			return fmt.Errorf("decoding inputmanifest %s/%s: %w", namespace, name, err)
		}

		state := im.Status.State
		clustersDone, clustersErr := len(im.Status.Clusters) > 0, false
		for c, s := range im.Status.Clusters {
			t.Logf("  cluster %s: state=%s phase=%s %s", c, s.State, s.Phase, s.Message)
			clustersDone = clustersDone && s.State == "DONE"
			clustersErr = clustersErr || s.State == "ERROR"
		}

		switch {
		case state == v1beta1.STATUS_ERROR || clustersErr:
			done, failed = 0, failed+1
		case state == v1beta1.STATUS_DONE,
			strings.HasPrefix(state, v1beta1.STATUS_WATCH) && clustersDone:
			done, failed = done+1, 0
		default:
			done, failed = 0, 0
		}
		t.Logf("inputmanifest %s/%s: state=%q (done %d/%d, error %d/%d)", namespace, name, state, done, needed, failed, needed)

		if done == needed {
			return nil
		}
		if failed == needed {
			return fmt.Errorf("inputmanifest %s/%s failed: %s", namespace, name, state)
		}
	}
}
