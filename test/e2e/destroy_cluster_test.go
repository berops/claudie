//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestDestroyCluster builds a cluster with dynamic and static nodes, deletes its InputManifest
// and checks that Claudie destroyed the dynamic nodes and cleaned up the static node.
func TestDestroyCluster(t *testing.T) {
	// Values must match manifests/destroy-cluster.yaml.
	const (
		clusterManifestFile           = "manifests/destroy-cluster.yaml"
		clusterName                   = "destroy-cluster-test-cluster"
		expectedNodeCount             = 3
		expectedDynamicNodeCount      = 2
		staticNodeEndpointPlaceholder = "/spec/nodePools/static/0/nodes/0/endpoint"
		staticNodeKeySecretName       = "static-nodes-key"
		secretsNamespace              = "e2e-secrets"
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)

	staticNodeIP := provisionStaticNodes(t)[0]
	manifest := replaceManifestValue(t, clusterManifestFile, staticNodeEndpointPlaceholder, staticNodeIP)
	manifest = applyInputManifest(t, e2eClusterClient, manifest, namespace)
	waitForInputManifest(t, e2eClusterClient, manifest, namespace, time.Hour)

	childClusterClient := newClient(t, childKubeconfig(t, e2eClusterClient, namespace, clusterName), kubernetes.NewForConfig)
	if got := readyNodeCount(t, childClusterClient); got != expectedNodeCount {
		t.Fatalf("cluster %s before deletion: got %d Ready nodes, want %d", clusterName, got, expectedNodeCount)
	}

	nodes := dynamicNodes(t, e2eClusterClient, namespace, clusterName)
	if got := len(nodes); got != expectedDynamicNodeCount {
		t.Fatalf("cluster %s before deletion: got %d dynamic nodes in metadata, want %d", clusterName, got, expectedDynamicNodeCount)
	}

	deleteInputManifest(t, e2eClusterClient, manifest, namespace, time.Hour)

	if got := reachableDynamicNodes(nodes); len(got) > 0 {
		t.Errorf("dynamic nodes after cluster deletion: got %v still reachable over SSH, want none", got)
	}

	// Claudie only logs a warning when it fails to clean up a static node and deletes the
	// InputManifest anyway, so only the node itself shows whether the cleanup worked.
	key := secretValue(t, e2eClusterClient, secretsNamespace, staticNodeKeySecretName, "privatekey")
	if got := staticNodeLeftovers(t, staticNodeIP, key); len(got) > 0 {
		t.Errorf("static node %s after cluster deletion: got leftovers %v, want none", staticNodeIP, got)
	}
}

// staticNodeInstallation is what Claudie puts on a static node and has to remove when the
// cluster is deleted, by roles/kubeadm_reset and wireguard-uninstall.yml in the ansibler.
// What the reset keeps on purpose, like the Longhorn packages, is not listed.
var staticNodeInstallation = []string{
	// Kubernetes and containerd packages.
	"/usr/bin/kubeadm", "/usr/bin/kubelet", "/usr/bin/kubectl", "/usr/bin/containerd",
	// Cluster state and configuration.
	"/etc/kubernetes", "/var/lib/etcd", "/var/lib/kubelet", "/root/kubeadm",
	"/etc/containerd", "/var/lib/containerd", "/opt/cni", "/etc/cni",
	// Apt repositories and kernel settings.
	"/etc/apt/sources.list.d/kubernetes.sources", "/etc/apt/sources.list.d/docker.sources",
	"/etc/apt/keyrings/kubernetes.asc", "/etc/apt/keyrings/docker.asc",
	"/etc/modules-load.d/k8s.conf", "/etc/sysctl.d/99-kubernetes.conf",
	// WireGuard, the interface is in /sys/class/net while it exists.
	"/etc/wireguard", "/usr/bin/wg", "/sys/class/net/wg0",
	// Claudie utilities.
	"/opt/claudie/bin", "/etc/profile.d/claudie_path.sh",
}

// reachableDynamicNodes returns the names of the nodes that still accept an SSH login with
// their nodepool key. A server that got a released IP afterwards rejects the key.
func reachableDynamicNodes(nodes []dynamicNode) []string {
	var reachable []string
	for _, n := range nodes {
		if client, err := dialSSH(n.IP, n.Key); err == nil {
			client.Close()
			reachable = append(reachable, n.Name)
		}
	}
	return reachable
}

// staticNodeLeftovers returns the paths from staticNodeInstallation still on the node.
func staticNodeLeftovers(t *testing.T, ip string, key []byte) []string {
	t.Helper()

	out := runSSH(t, ip, key, "ls -d "+strings.Join(staticNodeInstallation, " ")+" 2>/dev/null || true")
	return strings.Fields(out)
}
