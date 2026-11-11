//go:build e2e

package e2e

import (
	"crypto/rand"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const apiServerPort = "6443"

// TestLoadBalancer builds a cluster with a load balancer for the API server, detaches the load
// balancer, attaches it again and replaces its DNS hostname. After each step it checks that
// the API endpoint of the cluster moved.
func TestLoadBalancer(t *testing.T) {
	// Values must match manifests/loadbalancer-test/*.yaml.
	const (
		withLoadBalancerManifestFile         = "manifests/loadbalancer-test/with-loadbalancer.yaml"
		withoutLoadBalancerManifestFile      = "manifests/loadbalancer-test/without-loadbalancer.yaml"
		withLoadBalancerHostnameManifestFile = "manifests/loadbalancer-test/with-loadbalancer-hostname.yaml"
		loadBalancerHostnamePlaceholder      = "/spec/loadBalancers/clusters/0/dns/hostname"
		clusterName                          = "loadbalancer-test-cluster"
		controlNodePool                      = "htz-ctrl-nodes"
		loadBalancerDNSZone                  = "claudie.org"
		expectedNodeCount                    = 4
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)

	// Unique per run, a fixed hostname would collide with other runs in the same DNS zone.
	newLoadBalancerHostname := "e2e-" + strings.ToLower(rand.Text())
	withNewHostnameManifestFile := replaceManifestValue(t, withLoadBalancerHostnameManifestFile, loadBalancerHostnamePlaceholder, newLoadBalancerHostname)

	testCases := []struct {
		name         string
		manifestFile string
		// wantAPIEndpointSuffix is the DNS zone of the load balancer, or its whole hostname
		// when the manifest sets it. Empty when the API endpoint has to be a control node.
		wantAPIEndpointSuffix string
	}{
		{"create with load balancer", withLoadBalancerManifestFile, "." + loadBalancerDNSZone},
		{"detach load balancer", withoutLoadBalancerManifestFile, ""},
		{"attach load balancer", withLoadBalancerManifestFile, "." + loadBalancerDNSZone},
		{"replace load balancer hostname", withNewHostnameManifestFile, newLoadBalancerHostname + "." + loadBalancerDNSZone},
	}

	for _, step := range testCases {
		manifest := applyInputManifest(t, e2eClusterClient, step.manifestFile, namespace)
		// Gives Claudie time to pick up the change, otherwise the old DONE state passes the wait.
		time.Sleep(time.Minute)
		waitForInputManifest(t, e2eClusterClient, manifest, namespace, time.Hour)

		t.Run(step.name, func(t *testing.T) {
			// Fetched in every step, Claudie replaces the kubeconfig when the API endpoint moves.
			childClusterKubeconfig := childKubeconfig(t, e2eClusterClient, namespace, clusterName)
			childClusterClient := newClient(t, childClusterKubeconfig, kubernetes.NewForConfig)
			if got := readyNodeCount(t, childClusterClient); got != expectedNodeCount {
				t.Errorf("cluster %s: got %d Ready nodes, want %d", clusterName, got, expectedNodeCount)
			}

			controlNodeIPs := nodePoolIPs(t, e2eClusterClient, namespace, clusterName, controlNodePool)
			kubeconfigApiEndpoint := apiServerHost(childClusterClient)
			if step.wantAPIEndpointSuffix == "" {
				if !slices.Contains(controlNodeIPs, kubeconfigApiEndpoint) {
					t.Errorf("cluster %s: got API endpoint %s, want one of the control node IPs %v", clusterName, kubeconfigApiEndpoint, controlNodeIPs)
				}
			} else if !strings.HasSuffix(kubeconfigApiEndpoint, step.wantAPIEndpointSuffix) {
				t.Errorf("cluster %s: got API endpoint %s, want a load balancer hostname ending in %s", clusterName, kubeconfigApiEndpoint, step.wantAPIEndpointSuffix)
			}

			kubeadmConfigApiEndpoint := kubeadmClusterConfig(t, childClusterClient).ControlPlaneEndpoint
			if got, want := kubeadmConfigApiEndpoint, net.JoinHostPort(kubeconfigApiEndpoint, apiServerPort); got != want {
				t.Errorf("cluster %s: got kubeadm-config controlPlaneEndpoint %s, want %s", clusterName, got, want)
			}
			// Cilium reaches the API server through the API endpoint, so this also covers the load balancer.
			if err := runCiliumConnectivityTestQuick(t, childClusterKubeconfig); err != nil {
				t.Errorf("cluster %s: cilium connectivity test: %v", clusterName, err)
			}
		})
	}
}

// apiServerHost returns the host, without the port, of the API server that client talks to.
func apiServerHost(client kubernetes.Interface) string {
	return client.CoreV1().RESTClient().Get().URL().Hostname()
}

// nodePoolIPs returns the sorted public IPs of the cluster's dynamic nodes in nodePool, named
// as in the InputManifest.
func nodePoolIPs(t *testing.T, client dynamic.Interface, namespace, cluster, nodePool string) []string {
	t.Helper()

	var ips []string
	for _, n := range dynamicNodes(t, client, namespace, cluster) {
		if strings.HasPrefix(n.NodePool, nodePool+"-") {
			ips = append(ips, n.IP)
		}
	}
	slices.Sort(ips)
	return ips
}

// kubeadmClusterConfiguration holds the fields the tests check in the ClusterConfiguration
// that kubeadm keeps in the kubeadm-config ConfigMap.
type kubeadmClusterConfiguration struct {
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint"`
	APIServer            struct {
		CertSANs []string `json:"certSANs"`
	} `json:"apiServer"`
}

// kubeadmClusterConfig returns the ClusterConfiguration of the cluster behind client, with its
// certSANs sorted.
func kubeadmClusterConfig(t *testing.T, client kubernetes.Interface) kubeadmClusterConfiguration {
	t.Helper()

	configMap, err := retryK8sCall(func() (*corev1.ConfigMap, error) {
		return client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(t.Context(), "kubeadm-config", metav1.GetOptions{})
	})
	if err != nil {
		t.Fatalf("getting configmap kubeadm-config: %v", err)
	}
	var cfg kubeadmClusterConfiguration
	if err := yaml.Unmarshal([]byte(configMap.Data["ClusterConfiguration"]), &cfg); err != nil {
		t.Fatalf("decoding ClusterConfiguration of configmap kubeadm-config: %v", err)
	}
	slices.Sort(cfg.APIServer.CertSANs)
	return cfg
}
