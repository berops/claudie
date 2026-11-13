//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"

	"github.com/berops/claudie/api/crd/inputmanifest/v1beta1"
	"golang.org/x/crypto/ssh"
)

const (
	// fieldManager owns the fields the e2e tests set with server-side apply.
	fieldManager = "claudie-e2e"
	pollInterval = 15 * time.Second
	// inputManifestBuildTimeout bounds how long Claudie may take to build an InputManifest.
	inputManifestBuildTimeout = time.Hour
	// sshConnectTimeout bounds both the TCP connect and the SSH handshake.
	sshConnectTimeout = 30 * time.Second
)

var (
	inputManifests = v1beta1.GroupVersion.WithResource("inputmanifests")
	secrets        = corev1.SchemeGroupVersion.WithResource("secrets")

	// retryBackoff makes the first attempt and three retries.
	retryBackoff = wait.Backoff{Steps: 4, Duration: 5 * time.Second, Factor: 2}
)

// retryCall runs call and retries it on a retriable error, as calls to a cluster sometimes fail.
func retryCall(call func() error) error {
	return retry.OnError(retryBackoff, isRetriable, call)
}

// retryValue is retryCall for calls that also return a value.
func retryValue[T any](call func() (T, error)) (T, error) {
	var value T
	err := retryCall(func() (err error) {
		value, err = call()
		return err
	})
	return value, err
}

// isRetriable reports whether a failed call can succeed when repeated. A cancelled call and
// a request the API server rejected as malformed or not allowed cannot.
func isRetriable(err error) bool {
	return !errors.Is(err, context.Canceled) &&
		!apierrors.IsInvalid(err) && !apierrors.IsBadRequest(err) && !apierrors.IsForbidden(err)
}

// ignoreNotFound keeps retryCall from retrying a call whose object is already gone.
func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// replaceManifestValue copies the manifest in path to a temp file with the value at the JSON
// Pointer (RFC 6901) replaced and returns the copy's path. The field must already exist.
func replaceManifestValue(t *testing.T, path, pointer string, value any) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decoding manifest %s: %v", path, err)
	}
	if err := replaceAt(doc, pointer, value); err != nil {
		t.Fatalf("manifest %s: %v", path, err)
	}

	// JSON is valid YAML, so the copy applies like the original.
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("encoding manifest %s: %v", path, err)
	}
	copyPath := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(copyPath, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return copyPath
}

// replaceAt sets the existing value at the JSON Pointer in doc.
func replaceAt(doc any, pointer string, value any) error {
	if !strings.HasPrefix(pointer, "/") {
		return fmt.Errorf("pointer %q must start with /", pointer)
	}
	unescape := strings.NewReplacer("~1", "/", "~0", "~")
	keys := strings.Split(pointer[1:], "/")

	node := doc
	for i, raw := range keys {
		key, last := unescape.Replace(raw), i == len(keys)-1
		switch n := node.(type) {
		case map[string]any:
			if _, ok := n[key]; !ok {
				return fmt.Errorf("%s: no field %q", pointer, key)
			}
			if last {
				n[key] = value
				return nil
			}
			node = n[key]
		case []any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(n) {
				return fmt.Errorf("%s: no index %q in a list of %d", pointer, key, len(n))
			}
			if last {
				n[idx] = value
				return nil
			}
			node = n[idx]
		default:
			return fmt.Errorf("%s: cannot descend into %q, it is not an object or a list", pointer, key)
		}
	}
	return nil
}

// claudieNamespace returns the namespace Claudie is deployed in, from CLAUDIE_NAMESPACE.
// A missing value fails the test, a skipped test would pass in CI and hide a broken setup.
func claudieNamespace(t *testing.T) string {
	t.Helper()

	namespace := os.Getenv("CLAUDIE_NAMESPACE")
	if namespace == "" {
		t.Fatal("CLAUDIE_NAMESPACE must be set to the namespace Claudie is deployed in")
	}
	return namespace
}

// e2eClusterKubeconfig returns the path of the kubeconfig of the e2e cluster, the management
// cluster Claudie runs in, from KUBECONFIG.
func e2eClusterKubeconfig(t *testing.T) string {
	t.Helper()

	path := os.Getenv("KUBECONFIG")
	if path == "" {
		t.Fatal("KUBECONFIG must be set to the kubeconfig file of the management cluster")
	}
	return path
}

// newClient returns a client for the cluster in the kubeconfig file, built by newForConfig,
// e.g. dynamic.NewForConfig or kubernetes.NewForConfig.
func newClient[T any](t *testing.T, kubeconfig string, newForConfig func(*rest.Config) (T, error)) T {
	t.Helper()

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("loading kubeconfig %q: %v", kubeconfig, err)
	}
	client, err := newForConfig(cfg)
	if err != nil {
		t.Fatalf("creating client for kubeconfig %q: %v", kubeconfig, err)
	}
	return client
}

// applyInputManifest server-side applies the InputManifest in path, creating or updating it, and
// returns its name. The InputManifest and its infrastructure are deleted when the test ends.
func applyInputManifest(t *testing.T, client dynamic.Interface, path, namespace string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading inputmanifest: %v", err)
	}
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal(data, &obj.Object); err != nil {
		t.Fatalf("decoding inputmanifest %s: %v", path, err)
	}
	if obj.GetName() == "" {
		t.Fatalf("no metadata.name in %s", path)
	}
	obj.SetNamespace(namespace)

	t.Cleanup(func() { deleteInputManifest(t, client, obj.GetName(), namespace, time.Hour) })
	// Force takes over fields set by an earlier kubectl apply or another manager.
	_, err = retryValue(func() (*unstructured.Unstructured, error) {
		return client.Resource(inputManifests).Namespace(namespace).
			Apply(t.Context(), obj.GetName(), &obj, metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
	})
	if err != nil {
		t.Fatalf("applying inputmanifest %s: %v", obj.GetName(), err)
	}
	return obj.GetName()
}

// waitForInputManifest waits until Claudie has built the InputManifest and fails the test on an
// error or timeout. The status can flip, so a state counts once it held for stablePolls polls.
func waitForInputManifest(t *testing.T, client dynamic.Interface, name, namespace string) {
	t.Helper()

	const stablePolls = 8
	var done, failed int
	err := wait.PollUntilContextTimeout(t.Context(), pollInterval, inputManifestBuildTimeout, false, func(ctx context.Context) (bool, error) {
		obj, err := client.Resource(inputManifests).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Logf("getting inputmanifest %s: %v", name, err)
			return false, nil
		}
		var im v1beta1.InputManifest
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &im); err != nil {
			return false, fmt.Errorf("decoding inputmanifest %s: %w", name, err)
		}

		isDone, isFailed := inputManifestOutcome(t, im)
		switch {
		case isFailed:
			done, failed = 0, failed+1
		case isDone:
			done, failed = done+1, 0
		default:
			done, failed = 0, 0
		}
		t.Logf("inputmanifest %s: state=%q (done %d/%d, error %d/%d)", name, im.Status.State, done, stablePolls, failed, stablePolls)

		if failed == stablePolls {
			return false, fmt.Errorf("inputmanifest %s failed: %s", name, im.Status.State)
		}
		return done == stablePolls, nil
	})
	if err != nil {
		t.Fatalf("waiting for inputmanifest %s: %v", name, err)
	}
}

// inputManifestOutcome reports whether Claudie finished building im or failed, and logs the
// state of each cluster.
func inputManifestOutcome(t *testing.T, im v1beta1.InputManifest) (done, failed bool) {
	clustersDone := len(im.Status.Clusters) > 0
	for name, c := range im.Status.Clusters {
		t.Logf("  cluster %s: state=%s phase=%s %s", name, c.State, c.Phase, c.Message)
		clustersDone = clustersDone && c.State == "DONE"
		failed = failed || c.State == "ERROR"
	}

	state := im.Status.State
	// Claudie appends a message to the watch state when it cannot start, e.g. on a missing secret.
	blocked := state != v1beta1.STATUS_WATCH && strings.HasPrefix(state, v1beta1.STATUS_WATCH)
	failed = failed || blocked || state == v1beta1.STATUS_ERROR
	done = !failed && (state == v1beta1.STATUS_DONE ||
		state == v1beta1.STATUS_WATCH && clustersDone)
	return done, failed
}

// deleteInputManifest deletes the InputManifest and waits until Claudie has destroyed its
// infrastructure and removed the finalizer. A missing InputManifest is not an error.
func deleteInputManifest(t *testing.T, client dynamic.Interface, name, namespace string, timeout time.Duration) {
	t.Helper()

	// Not t.Context, it is already cancelled when a cleanup calls this.
	ctx := context.Background()
	res := client.Resource(inputManifests).Namespace(namespace)

	existing, err := retryValue(func() (*unstructured.Unstructured, error) {
		im, err := res.Get(ctx, name, metav1.GetOptions{})
		return im, ignoreNotFound(err)
	})
	if err != nil {
		t.Fatalf("getting inputmanifest %s: %v", name, err)
	}
	if existing == nil {
		return
	}
	// An earlier call already waited for this deletion, e.g. the cleanup of another apply.
	if existing.GetDeletionTimestamp() != nil {
		t.Fatalf("inputmanifest %s is still being deleted", name)
	}

	err = retryCall(func() error { return ignoreNotFound(res.Delete(ctx, name, metav1.DeleteOptions{})) })
	if err != nil {
		t.Fatalf("deleting inputmanifest %s: %v", name, err)
	}

	err = wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := res.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			t.Logf("getting inputmanifest %s: %v", name, err)
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("inputmanifest %s still exists: %v", name, err)
	}
}

// outputSecret returns the Secret Claudie keeps for a cluster it built in its namespace, labeled
// claudie.io/output=<output> and claudie.io/cluster=<cluster>.
func outputSecret(t *testing.T, client dynamic.Interface, namespace, cluster, output string) corev1.Secret {
	t.Helper()

	selector := labels.SelectorFromSet(labels.Set{
		"claudie.io/output":  output,
		"claudie.io/cluster": cluster,
	}).String()
	list, err := retryValue(func() (*unstructured.UnstructuredList, error) {
		return client.Resource(secrets).Namespace(namespace).List(t.Context(), metav1.ListOptions{LabelSelector: selector})
	})
	if err != nil {
		t.Fatalf("listing %s secrets of cluster %s: %v", output, cluster, err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("want 1 secret with %s in %s, got %d", selector, namespace, len(list.Items))
	}

	// The converter base64-decodes the Secret data.
	var secret corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, &secret); err != nil {
		t.Fatalf("decoding secret %s: %v", list.Items[0].GetName(), err)
	}
	return secret
}

// childKubeconfig writes the kubeconfig of a cluster built by Claudie to a temp file and
// returns its path. Claudie keeps it in the kubeconfig output Secret in its namespace.
func childKubeconfig(t *testing.T, client dynamic.Interface, namespace, cluster string) string {
	t.Helper()

	secret := outputSecret(t, client, namespace, cluster, "kubeconfig")
	kubeconfig := secret.Data["kubeconfig"]
	if len(kubeconfig) == 0 {
		t.Fatalf("secret %s has no kubeconfig key", secret.Name)
	}

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, kubeconfig, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// dynamicNode is a node of the Kubernetes cluster that Claudie created on a cloud provider.
type dynamicNode struct {
	Name string
	IP   string
	// NodePool is the nodepool name from the InputManifest with the hash Claudie appends.
	NodePool string
	// Key is the private SSH key of the nodepool of the node.
	Key []byte
}

// dynamicNodes returns the dynamic nodes of a cluster built by Claudie. Claudie keeps them in
// the metadata output Secret in its namespace and deletes the Secret with the cluster.
func dynamicNodes(t *testing.T, client dynamic.Interface, namespace, cluster string) []dynamicNode {
	t.Helper()

	secret := outputSecret(t, client, namespace, cluster, "metadata")
	// Only the fields used here, the full type is ClusterMetadata in
	// services/manager/internal/service/managementcluster.
	var md struct {
		DynamicNodepools map[string]struct {
			NodeIps map[string]struct {
				PublicIP string `json:"public_ip"`
			} `json:"node_ips"`
			PrivateKey string `json:"nodepool_private_key"`
		} `json:"dynamic_nodepools"`
	}
	if err := json.Unmarshal(secret.Data["metadata"], &md); err != nil {
		t.Fatalf("decoding metadata of cluster %s from secret %s: %v", cluster, secret.Name, err)
	}

	var nodes []dynamicNode
	for nodePool, np := range md.DynamicNodepools {
		for name, ips := range np.NodeIps {
			nodes = append(nodes, dynamicNode{Name: name, IP: ips.PublicIP, NodePool: nodePool, Key: []byte(np.PrivateKey)})
		}
	}
	return nodes
}

// secretValue returns the key field of the Secret, failing the test if it is missing or empty.
func secretValue(t *testing.T, client dynamic.Interface, namespace, name, key string) []byte {
	t.Helper()

	obj, err := retryValue(func() (*unstructured.Unstructured, error) {
		return client.Resource(secrets).Namespace(namespace).Get(t.Context(), name, metav1.GetOptions{})
	})
	if err != nil {
		t.Fatalf("getting secret %s/%s: %v", namespace, name, err)
	}
	// The converter base64-decodes the Secret data.
	var secret corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &secret); err != nil {
		t.Fatalf("decoding secret %s/%s: %v", namespace, name, err)
	}
	value := secret.Data[key]
	if len(value) == 0 {
		t.Fatalf("secret %s/%s has no %s field", namespace, name, key)
	}
	return value
}

// runSSH runs command on host as root over SSH with the private key and returns its stdout.
// A non-zero exit code fails the test.
func runSSH(t *testing.T, host string, key []byte, command string) string {
	t.Helper()

	client, err := retryValue(func() (*ssh.Client, error) { return dialSSH(host, key) })
	if err != nil {
		t.Fatalf("connecting to %s: %v", host, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("opening ssh session to %s: %v", host, err)
	}
	defer session.Close()

	out, err := session.Output(command)
	if err != nil {
		t.Fatalf("running %q on %s: %v", command, host, err)
	}
	return string(out)
}

// dialSSH logs in to host as root with the private key.
func dialSSH(host string, key []byte) (*ssh.Client, error) {
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parsing ssh key: %w", err)
	}
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		// The tests connect to nodes they created, there is no known host key to check.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := net.JoinHostPort(host, "22")
	conn, err := (&net.Dialer{Timeout: sshConnectTimeout}).DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	// ssh.Dial bounds only the TCP connect, the deadline bounds the handshake too.
	if err := conn.SetDeadline(time.Now().Add(sshConnectTimeout)); err != nil {
		conn.Close()
		return nil, err
	}
	sshConn, channels, requests, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		sshConn.Close()
		return nil, err
	}
	return ssh.NewClient(sshConn, channels, requests), nil
}

// listNodes returns the nodes of the cluster behind client.
func listNodes(t *testing.T, client kubernetes.Interface) []corev1.Node {
	t.Helper()

	nodes, err := retryValue(func() (*corev1.NodeList, error) {
		return client.CoreV1().Nodes().List(t.Context(), metav1.ListOptions{})
	})
	if err != nil {
		t.Fatalf("listing nodes: %v", err)
	}
	return nodes.Items
}

// nodeCount returns how many nodes the cluster behind client has, Ready or not.
func nodeCount(t *testing.T, client kubernetes.Interface) int {
	t.Helper()
	return len(listNodes(t, client))
}

// notReadyNodes returns the names of the nodes of the cluster behind client that are not Ready.
func notReadyNodes(t *testing.T, client kubernetes.Interface) []string {
	t.Helper()

	var notReady []string
	for _, n := range listNodes(t, client) {
		if !isNodeReady(n) {
			notReady = append(notReady, n.Name)
		}
	}
	return notReady
}

func isNodeReady(n corev1.Node) bool {
	return slices.ContainsFunc(n.Status.Conditions, func(c corev1.NodeCondition) bool {
		return c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue
	})
}

// provisionStaticNodes creates the static nodes with the tofu script in STATIC_NODES_TF and
// returns their IPs. The nodes are destroyed when the test ends. The tofu state is kept in
// STATIC_NODES_DIR, where the CI destroys the nodes of a test that timed out or was cancelled.
func provisionStaticNodes(t *testing.T) []string {
	t.Helper()

	script, err := base64.StdEncoding.DecodeString(os.Getenv("STATIC_NODES_TF"))
	if err != nil {
		t.Fatalf("STATIC_NODES_TF is not valid base64: %v", err)
	}
	if len(script) == 0 {
		t.Fatal("STATIC_NODES_TF must be set to the base64 encoded tofu script creating the static nodes")
	}

	dir := os.Getenv("STATIC_NODES_DIR")
	if dir == "" {
		t.Fatal("STATIC_NODES_DIR must be set to the directory to keep the tofu script and state in")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), script, 0o600); err != nil {
		t.Fatal(err)
	}

	// Registered before apply, so a partially applied script is destroyed too. It runs after the
	// cleanups registered later, e.g. deleting the InputManifest that uses the nodes.
	t.Cleanup(func() {
		// Not t.Context, it is already cancelled when cleanups run.
		if err := runTofu(context.Background(), dir, "destroy", "-auto-approve"); err != nil {
			t.Errorf("destroying static nodes: %v", err)
		}
	})

	if err := runTofu(t.Context(), dir, "init", "-input=false"); err != nil {
		t.Fatal(err)
	}
	if err := runTofu(t.Context(), dir, "apply", "-auto-approve", "-input=false"); err != nil {
		t.Fatal(err)
	}

	ips, err := staticNodeIPs(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return ips
}

// staticNodeIPs returns the IPs of the "endpoints" tofu output, which is either a list or a map.
// The IPs of a map are ordered by the node names, so the same script gives the same order.
func staticNodeIPs(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "tofu", "output", "-json", "endpoints")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tofu output endpoints: %w", err)
	}

	var ips []string
	if err := json.Unmarshal(out, &ips); err != nil {
		var byName map[string]string
		if err := json.Unmarshal(out, &byName); err != nil {
			return nil, fmt.Errorf("tofu output endpoints is neither a list nor a map of IPs: %s", out)
		}
		for _, name := range slices.Sorted(maps.Keys(byName)) {
			ips = append(ips, byName[name])
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP in tofu output endpoints: %s", out)
	}
	return ips, nil
}

func runTofu(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "tofu", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tofu %v failed: %w", args, err)
	}
	return nil
}

// runCiliumConnectivityTestFull runs every test cilium connectivity test has. It is slow, so
// only the conformance test runs it.
func runCiliumConnectivityTestFull(t *testing.T, kubeconfig string) error {
	t.Helper()
	return runCiliumConnectivityTest(t, kubeconfig, "--timeout", "60m")
}

// runCiliumConnectivityTestQuick runs only ciliumQuickTests, a basic connectivity check.
func runCiliumConnectivityTestQuick(t *testing.T, kubeconfig string) error {
	t.Helper()
	// ciliumQuickTests check pod, service, NodePort, host and external traffic and cilium-health.
	// --test matches "<test>/<scenario>", so the anchors keep no-policies off no-policies-extra.
	var ciliumQuickTests = []string{"^no-policies/", "^no-policies-extra/", "^health/"}
	return runCiliumConnectivityTest(t, kubeconfig, "--test", strings.Join(ciliumQuickTests, ","), "--timeout", "10m")
}

// runCiliumConnectivityTest waits until Cilium is ready and runs cilium connectivity test with
// args. The test namespaces are deleted when the test ends.
func runCiliumConnectivityTest(t *testing.T, kubeconfig string, args ...string) error {
	t.Helper()

	// Both commands use the default --test-namespace, the only one --cleanup deletes.
	t.Cleanup(func() {
		// Not t.Context, it is already cancelled when cleanups run.
		if err := runCLI(context.Background(), t, kubeconfig, "cilium", "connectivity", "test", "--cleanup"); err != nil {
			t.Errorf("cleaning up cilium connectivity test: %v", err)
		}
	})

	if err := runCLI(t.Context(), t, kubeconfig, "cilium", "status", "--wait", "--wait-duration", "10m", "--interactive=false"); err != nil {
		return fmt.Errorf("waiting for cilium to be ready: %w", err)
	}
	// Hubble flow validation is off. Cilium runs with monitor aggregation "medium"
	// (services/kuber/manifests/cilium.yaml), which skips most flow checks anyway.
	return runCLI(t.Context(), t, kubeconfig, "cilium", append([]string{"connectivity", "test", "--hubble=false"}, args...)...)
}

// runCLI runs a CLI against the cluster in kubeconfig and streams its output.
func runCLI(ctx context.Context, t *testing.T, kubeconfig, name string, args ...string) error {
	t.Logf("running: %s %s", name, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
