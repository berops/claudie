//go:build e2e

package e2e

import (
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

func TestAPIEndpointMoves(t *testing.T) {
	const (
		withLoadBalancerManifestFile           = "manifests/api-endpoint-moves/with-loadbalancer.yaml"
		withoutLoadBalancerManifestFile        = "manifests/api-endpoint-moves/without-loadbalancer.yaml"
		withoutEndpointControlNodeManifestFile = "manifests/api-endpoint-moves/without-endpoint-control-node.yaml"
		withLoadBalancerHostnameManifestFile   = "manifests/api-endpoint-moves/with-loadbalancer-hostname.yaml"
		clusterName                            = "api-endpoint-moves-cluster"
		// Listed first in pools.control, so detaching the load balancer moves the API endpoint here.
		firstControlNodePool  = "htz-ctrl-a"
		secondControlNodePool = "htz-ctrl-b"
		loadBalancerDNSZone   = "claudie.org"
	)

	namespace := claudieNamespace(t)
	e2eClusterClient := newClient(t, e2eClusterKubeconfig(t), dynamic.NewForConfig)

	testCases := []struct {
		name                    string
		manifestFile            string
		wantNodeCount           int
		wantAPIEndpointNodePool string
		wantAPIEndpointSuffix   string
	}{
		{
			name:                  "create cluster with load balancer attached",
			manifestFile:          withLoadBalancerManifestFile,
			wantNodeCount:         4,
			wantAPIEndpointSuffix: loadBalancerDNSZone,
		},
		{
			name:                    "detach load balancer",
			manifestFile:            withoutLoadBalancerManifestFile,
			wantNodeCount:           4,
			wantAPIEndpointNodePool: firstControlNodePool,
		},
		{
			name:                    "delete API endpoint control node",
			manifestFile:            withoutEndpointControlNodeManifestFile,
			wantNodeCount:           3,
			wantAPIEndpointNodePool: secondControlNodePool,
		},
		{
			name:                    "add control node back",
			manifestFile:            withoutLoadBalancerManifestFile,
			wantNodeCount:           4,
			wantAPIEndpointNodePool: secondControlNodePool,
		},
		{
			name:                  "attach load balancer",
			manifestFile:          withLoadBalancerManifestFile,
			wantNodeCount:         4,
			wantAPIEndpointSuffix: loadBalancerDNSZone,
		},
		{
			name:                  "replace load balancer hostname",
			manifestFile:          withLoadBalancerHostnameManifestFile,
			wantNodeCount:         4,
			wantAPIEndpointSuffix: namespace + "." + loadBalancerDNSZone,
		},
	}

	for _, step := range testCases {
		manifest := applyInputManifest(t, e2eClusterClient, step.manifestFile, namespace)
		// Gives Claudie time to pick up the change, otherwise the old DONE state passes the wait.
		time.Sleep(time.Minute)
		waitForInputManifest(t, e2eClusterClient, manifest, namespace)

		t.Run(step.name, func(t *testing.T) {
			// Fetched in every step, Claudie replaces the kubeconfig when the API endpoint moves.
			childClusterKubeconfig := childKubeconfig(t, e2eClusterClient, namespace, clusterName)
			childClusterClient := newClient(t, childClusterKubeconfig, kubernetes.NewForConfig)
			if got := nodeCount(t, childClusterClient); got != step.wantNodeCount {
				t.Errorf("cluster %s: got %d nodes, want %d", clusterName, got, step.wantNodeCount)
			}
			if got := notReadyNodes(t, childClusterClient); len(got) > 0 {
				t.Errorf("cluster %s: got not Ready nodes %v, want none", clusterName, got)
			}

			kubeconfigApiEndpoint := apiServerHost(childClusterClient)
			if step.wantAPIEndpointNodePool != "" {
				if ips := nodePoolIPs(t, e2eClusterClient, namespace, clusterName, step.wantAPIEndpointNodePool); !slices.Contains(ips, kubeconfigApiEndpoint) {
					t.Errorf("cluster %s: got API endpoint %s, want one of the %s node IPs %v", clusterName, kubeconfigApiEndpoint, step.wantAPIEndpointNodePool, ips)
				}
			} else if !strings.HasSuffix("."+kubeconfigApiEndpoint, "."+step.wantAPIEndpointSuffix) {
				t.Errorf("cluster %s: got API endpoint %s, want a load balancer hostname ending in DNS labels %s", clusterName, kubeconfigApiEndpoint, step.wantAPIEndpointSuffix)
			}

			kubeadmConfig := kubeadmClusterConfig(t, childClusterClient)
			if got, want := kubeadmConfig.ControlPlaneEndpoint, net.JoinHostPort(kubeconfigApiEndpoint, apiServerPort); got != want {
				t.Errorf("cluster %s: got kubeadm-config controlPlaneEndpoint %s, want %s", clusterName, got, want)
			}
			controlNodeIPs := nodePoolIPs(t, e2eClusterClient, namespace, clusterName, firstControlNodePool, secondControlNodePool)
			// kubeadm join issues the API server certificate of a control node for these.
			if got := kubeadmConfig.APIServer.CertSANs; !slices.Equal(got, controlNodeIPs) {
				t.Errorf("cluster %s: got kubeadm-config certSANs %v, want the control node IPs %v", clusterName, got, controlNodeIPs)
			}
			wantCertNames := append([]string{kubeconfigApiEndpoint}, controlNodeIPs...)
			if got := unverifiedAPIServerNames(t, e2eClusterClient, namespace, clusterName, controlNodeIPs, wantCertNames); len(got) > 0 {
				t.Errorf("cluster %s: got served API server certificates not verifying for %v by control node, want all verifying for %v", clusterName, got, wantCertNames)
			}
			// Cilium reaches the API server through the API endpoint, so this also covers the load balancer.
			if err := runCiliumConnectivityTestQuick(t, childClusterKubeconfig); err != nil {
				t.Errorf("cluster %s: cilium connectivity test: %v", clusterName, err)
			}
		})
		if t.Failed() {
			t.FailNow()
		}
	}
}

// apiServerHost returns the host, without the port, of the API server that client talks to.
func apiServerHost(client kubernetes.Interface) string {
	return client.CoreV1().RESTClient().Get().URL().Hostname()
}

// nodePoolIPs returns the sorted public IPs of the cluster's dynamic nodes in nodePools, named
// as in the InputManifest.
func nodePoolIPs(t *testing.T, client dynamic.Interface, namespace, cluster string, nodePools ...string) []string {
	t.Helper()

	var ips []string
	for _, n := range dynamicNodes(t, client, namespace, cluster) {
		if slices.ContainsFunc(nodePools, func(nodePool string) bool { return strings.HasPrefix(n.NodePool, nodePool+"-") }) {
			ips = append(ips, n.IP)
		}
	}
	slices.Sort(ips)
	return ips
}

// unverifiedAPIServerNames returns, by node name, which of names the certificate served by the API
// server of the nodes with nodeIPs does not verify for with the cluster CA. Nodes that pass are left out.
func unverifiedAPIServerNames(t *testing.T, client dynamic.Interface, namespace, cluster string, nodeIPs, names []string) map[string][]string {
	t.Helper()

	unverified := map[string][]string{}
	for _, n := range dynamicNodes(t, client, namespace, cluster) {
		if !slices.Contains(nodeIPs, n.IP) {
			continue
		}
		for _, name := range names {
			verify := "kubectl --kubeconfig /etc/kubernetes/admin.conf --server https://127.0.0.1:" + apiServerPort +
				" --tls-server-name " + name + " --request-timeout 10s get --raw /readyz >/dev/null 2>&1 || echo unverified"
			if runSSH(t, n.IP, dynamicNodeSSHPort, n.Key, verify) != "" {
				unverified[n.Name] = append(unverified[n.Name], name)
			}
		}
	}
	return unverified
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

	configMap, err := retryValue(func() (*corev1.ConfigMap, error) {
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
