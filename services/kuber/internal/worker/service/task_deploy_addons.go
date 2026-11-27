package service

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	metricsServerYaml = "services/kuber/manifests/metrics-server.yaml"
	nodeLocalDNSYaml  = "services/kuber/manifests/nodelocaldns.yaml"
	coreDNSPDBYaml    = "services/kuber/manifests/coredns-pdb.yaml"
)

// DeployAddons deploys the cluster addons previously installed by kubeone.
// Must run after Cilium, the addons need a working CNI.
func DeployAddons(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Deploying cluster addons")

	var k8s *spec.K8Scluster

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = do.Create.K8S
	case *spec.Task_Update:
		k8s = do.Update.State.K8S
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to deploy cluster addons, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	if err := deployAddons(logger, k8s); err != nil {
		logger.Err(err).Msg("Failed to deploy cluster addons")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Cluster addons successfully deployed")
}

func deployAddons(logger zerolog.Logger, k8s *spec.K8Scluster) error {
	k, err := clusterKubectl(k8s)
	if err != nil {
		return err
	}

	dir, cleanup, err := manifestDirectory(k8s.ClusterInfo.Id())
	if err != nil {
		return err
	}
	defer cleanup()

	// First, the kubelets point pods to the cache, without it the cluster has no DNS.
	logger.Info().Msg("Deploying NodeLocal DNSCache")
	if err := deployNodeLocalDNS(k, dir); err != nil {
		return fmt.Errorf("failed to deploy NodeLocal DNSCache: %w", err)
	}

	logger.Info().Msg("Deploying CoreDNS PodDisruptionBudget")
	if err := applyManifest(k, dir, coreDNSPDBYaml, nil); err != nil {
		return fmt.Errorf("failed to deploy CoreDNS PodDisruptionBudget: %w", err)
	}

	logger.Info().Msg("Deploying metrics-server")
	if err := deployMetricsServer(k, dir); err != nil {
		return fmt.Errorf("failed to deploy metrics-server: %w", err)
	}

	return nil
}

// deployMetricsServer applies the metrics-server manifest.
func deployMetricsServer(k kubectl.Kubectl, dir string) error {
	return applyManifest(k, dir, metricsServerYaml, nil)
}

// deployNodeLocalDNS applies the upstream NodeLocal DNSCache manifest with its placeholders filled in.
func deployNodeLocalDNS(k kubectl.Kubectl, dir string) error {
	// IP the cache listens on every node. A chosen link-local IP, not derived from the cluster:
	// upstream default, same as kubeone (NodeLocalDNSVirtualIP). The kubelets point pods to it,
	// must match node_local_dns_ip in ansibler/ansible-playbooks/vars/k8s.yaml.
	const nodeLocalDNSIP = "169.254.20.10"

	placeholders := map[string]string{
		// -localip argument (single key): listen only on nodeLocalDNSIP, not the kube-dns IP,
		// which Cilium's kube-proxy replacement handles
		"__PILLAR__LOCAL__DNS__,__PILLAR__DNS__SERVER__": nodeLocalDNSIP,
		// __PILLAR__LOCAL__DNS__: set, the liveness probe host is not filled in by node-cache.
		"__PILLAR__LOCAL__DNS__": nodeLocalDNSIP,
		// __PILLAR__DNS__DOMAIN__: set, node-cache never fills it. kubeadm default domain.
		"__PILLAR__DNS__DOMAIN__": "cluster.local",

		// __PILLAR__DNS__SERVER__: not set, node-cache replaces it with an empty string,
		// so the Corefile binds only nodeLocalDNSIP.
		// __PILLAR__CLUSTER__DNS__: not set, node-cache fills it with the kube-dns-upstream service IP.
		// __PILLAR__UPSTREAM__SERVERS__: not set, node-cache fills it from the node's resolv.conf.
	}

	return applyManifest(k, dir, nodeLocalDNSYaml, placeholders)
}

// clusterKubectl returns a kubectl client for the cluster.
func clusterKubectl(k8s *spec.K8Scluster) (kubectl.Kubectl, error) {
	clusterID := k8s.ClusterInfo.Id()

	if k8s.Kubeconfig == "" {
		return kubectl.Kubectl{}, fmt.Errorf("cluster %s has no kubeconfig, can't deploy cluster addons", clusterID)
	}

	k := kubectl.Kubectl{
		Kubeconfig:        k8s.Kubeconfig,
		MaxKubectlRetries: 3,
	}

	k.Stdout = comm.GetStdOut(clusterID)
	k.Stderr = comm.GetStdErr(clusterID)

	return k, nil
}

// manifestDirectory creates a temporary directory for the filled in manifests, they are applied
// from files as single quotes break [kubectl.Kubectl.KubectlApplyString]. cleanup removes it.
func manifestDirectory(clusterID string) (string, func(), error) {
	dir := filepath.Join(OutputDir, fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)))
	if err := fileutils.CreateDirectory(dir); err != nil {
		return "", nil, fmt.Errorf("error while creating directory %s when deploying cluster addons: %w", dir, err)
	}

	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Err(err).Msgf("Failed to remove directory %s where the addon manifests were rendered", dir)
		}
	}

	return dir, cleanup, nil
}

// applyManifest fills in the placeholders of the manifest, writes it into dir and applies it.
func applyManifest(k kubectl.Kubectl, dir, manifest string, placeholders map[string]string) error {
	content, err := os.ReadFile(manifest)
	if err != nil {
		return fmt.Errorf("error while reading manifest %s: %w", manifest, err)
	}

	filled, err := replacePlaceholders(string(content), placeholders)
	if err != nil {
		return fmt.Errorf("error while filling in manifest %s: %w", manifest, err)
	}

	path := filepath.Join(dir, filepath.Base(manifest))
	if err := os.WriteFile(path, []byte(filled), 0o600); err != nil {
		return fmt.Errorf("error while writing manifest to %s: %w", path, err)
	}

	if err := k.KubectlApply(path); err != nil {
		return fmt.Errorf("error while applying manifest %s: %w", manifest, err)
	}

	return nil
}

// replacePlaceholders replaces each placeholder with its value, longest first so that a
// placeholder containing others is replaced whole. A missing placeholder is an error,
// so that a changed upstream manifest is not applied half filled.
func replacePlaceholders(manifest string, placeholders map[string]string) (string, error) {
	keys := slices.SortedFunc(maps.Keys(placeholders), func(a, b string) int {
		return cmp.Compare(len(b), len(a))
	})

	for _, placeholder := range keys {
		if !strings.Contains(manifest, placeholder) {
			return "", fmt.Errorf("manifest does not contain placeholder %s, was the upstream manifest changed?", placeholder)
		}
		manifest = strings.ReplaceAll(manifest, placeholder, placeholders[placeholder])
	}

	return manifest, nil
}
