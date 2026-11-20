//go:build e2e

package e2e

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// TestUpgradeCluster builds a cluster, bumps its Kubernetes version and checks after each step that
// every node and the control plane run the wanted version and the cluster network works.
func TestUpgradeCluster(t *testing.T) {
	const (
		clusterFromManifestFile = "manifests/upgrade-cluster/from-version.yaml"
		clusterToManifestFile   = "manifests/upgrade-cluster/to-version.yaml"
		clusterName             = "upgrade-cluster-test-cluster"
		expectedNodeCount       = 4
		controlNodeCount        = 2
		// kube-apiserver, kube-controller-manager and kube-scheduler on each control node.
		expectedControlPlanePodCount = 3 * controlNodeCount
		upgradeFromVersion           = "v1.35.0"
		upgradeToVersion             = "v1.36.0"
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
		manifest, lastFinished := applyInputManifest(t, e2eClusterClient, step.manifestFile, namespace)
		waitForInputManifest(t, e2eClusterClient, manifest, namespace, lastFinished)

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
			pods := controlPlanePods(t, childClusterClient)
			if got := len(pods); got != expectedControlPlanePodCount {
				t.Errorf("cluster %s: got %d control plane pods, want %d", clusterName, got, expectedControlPlanePodCount)
			}
			if got, want := controlPlaneVersions(t, pods), []string{step.wantVersion}; !slices.Equal(got, want) {
				t.Errorf("cluster %s: got control plane versions %v, want %v", clusterName, got, want)
			}
			if got := notReadyControlPlanePods(t, childClusterClient); len(got) > 0 {
				t.Errorf("cluster %s: got not Ready control plane pods %v, want none", clusterName, got)
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

// controlPlanePods returns the kube-apiserver, kube-controller-manager and kube-scheduler pods
// of the cluster behind client.
func controlPlanePods(t *testing.T, client kubernetes.Interface) []corev1.Pod {
	t.Helper()

	// kubeadm labels its static pods with their component.
	const selector = "component in (kube-apiserver,kube-controller-manager,kube-scheduler)"
	pods, err := retryValue(func() (*corev1.PodList, error) {
		return client.CoreV1().Pods(metav1.NamespaceSystem).List(t.Context(), metav1.ListOptions{LabelSelector: selector})
	})
	if err != nil {
		t.Fatalf("listing control plane pods: %v", err)
	}
	return pods.Items
}

// controlPlaneVersions returns the sorted, distinct image versions of pods.
func controlPlaneVersions(t *testing.T, pods []corev1.Pod) []string {
	t.Helper()

	var versions []string
	for _, p := range pods {
		for _, c := range p.Spec.Containers {
			version := c.Image[strings.LastIndex(c.Image, ":")+1:]
			t.Logf("  pod %s: %s", p.Name, version)
			versions = append(versions, version)
		}
	}
	slices.Sort(versions)
	return slices.Compact(versions)
}

// notReadyControlPlanePods waits until the control plane pods of the cluster behind client stay
// Ready for stablePolls polls, as they restart after an upgrade, and returns the last not Ready ones.
func notReadyControlPlanePods(t *testing.T, client kubernetes.Interface) []string {
	t.Helper()

	const (
		timeout     = 5 * time.Minute
		stablePolls = 3
	)
	var lastNotReady []string
	readyPolls := 0
	err := wait.PollUntilContextTimeout(t.Context(), pollInterval, timeout, true, func(context.Context) (bool, error) {
		notReady := notReadyPods(controlPlanePods(t, client))
		if len(notReady) > 0 {
			lastNotReady, readyPolls = notReady, 0
		} else {
			readyPolls++
		}
		t.Logf("control plane pods: not Ready %v (ready %d/%d)", notReady, readyPolls, stablePolls)
		return readyPolls == stablePolls, nil
	})
	if err != nil {
		return lastNotReady
	}
	return nil
}

// notReadyPods returns the names of pods that are not Ready, e.g. with a crash-looping container.
func notReadyPods(pods []corev1.Pod) []string {
	var notReady []string
	for _, p := range pods {
		if !isPodReady(p) {
			notReady = append(notReady, p.Name)
		}
	}
	return notReady
}

func isPodReady(p corev1.Pod) bool {
	return slices.ContainsFunc(p.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
	})
}
