package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/clusters"
	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// ciliumYaml is the Cilium manifest rendered with helm, see the header of the file
	// for the values used and how to regenerate it.
	ciliumYaml = "services/kuber/manifests/cilium.yaml"

	// ciliumAPIEndpointPlaceholder is the value of k8sServiceHost the manifest was rendered
	// with. With kube-proxy replacement Cilium can't reach the API server via the kubernetes
	// service, as Cilium itself implements it, thus it is replaced with the API endpoint of
	// the cluster before applying. The endpoint comes from [clusters.ControlPlaneEndpointHost],
	// the same function the ansibler uses for the kubeadm controlPlaneEndpoint.
	//
	// k8sServiceHost=auto of the chart is not an option, it is resolved by a helm lookup of
	// the kube-public/cluster-info ConfigMap at render time, which fails with helm template
	// and would not follow later moves of the API endpoint either.
	ciliumAPIEndpointPlaceholder = "claudie-api-endpoint-replace-me"
)

// DeployCilium deploys, or updates, the Cilium CNI on the cluster. Applying the manifest
// again after the API endpoint changed updates the API server address used by Cilium,
// which rolls out the Cilium pods with the new address.
func DeployCilium(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Deploying CNI Cilium")

	var (
		k8s *spec.K8Scluster
		lbs []*spec.LBcluster
	)

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = do.Create.K8S
		lbs = do.Create.LoadBalancers
	case *spec.Task_Update:
		k8s = do.Update.State.K8S
		lbs = do.Update.State.LoadBalancers
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to deploy Cilium, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	if err := deployCilium(k8s, lbs); err != nil {
		logger.Err(err).Msg("Failed to deploy Cilium")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Cilium successfully deployed")
}

func deployCilium(k8s *spec.K8Scluster, lbs []*spec.LBcluster) error {
	clusterID := k8s.ClusterInfo.Id()

	if k8s.Kubeconfig == "" {
		return fmt.Errorf("cluster %s has no kubeconfig, can't deploy Cilium", clusterID)
	}

	endpoint, err := clusters.ControlPlaneEndpointHost(k8s, lbs)
	if err != nil {
		return fmt.Errorf("failed to determine the api endpoint of %s for Cilium: %w", clusterID, err)
	}

	manifest, err := os.ReadFile(ciliumYaml)
	if err != nil {
		return fmt.Errorf("error while reading %s: %w", ciliumYaml, err)
	}

	if !strings.Contains(string(manifest), ciliumAPIEndpointPlaceholder) {
		return fmt.Errorf("%s does not contain the api endpoint placeholder %q, was it rendered with k8sServiceHost set to it?", ciliumYaml, ciliumAPIEndpointPlaceholder)
	}

	rendered := strings.ReplaceAll(string(manifest), ciliumAPIEndpointPlaceholder, endpoint)

	// The manifest contains single quotes, which breaks [kubectl.Kubectl.KubectlApplyString],
	// thus it is written to a file and applied from there.
	clusterDir := filepath.Join(OutputDir, fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)))
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		return fmt.Errorf("error while creating directory %s when deploying Cilium: %w", clusterDir, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("Failed to remove directory %s where the Cilium manifest was rendered", clusterDir)
		}
	}()

	renderedPath := filepath.Join(clusterDir, "cilium.yaml")
	if err := os.WriteFile(renderedPath, []byte(rendered), 0o600); err != nil {
		return fmt.Errorf("error while writing the rendered Cilium manifest to %s: %w", renderedPath, err)
	}

	k := kubectl.Kubectl{
		Kubeconfig:        k8s.Kubeconfig,
		MaxKubectlRetries: 3,
	}

	k.Stdout = comm.GetStdOut(clusterID)
	k.Stderr = comm.GetStdErr(clusterID)

	if err := k.KubectlApply(renderedPath); err != nil {
		return fmt.Errorf("error while applying the Cilium manifest: %w", err)
	}

	return nil
}
