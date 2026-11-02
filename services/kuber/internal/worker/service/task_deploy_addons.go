package service

import (
	"fmt"
	"os"
	"path/filepath"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// addonManifests are the cluster addons previously installed by kubeone, now applied by
// kuber for clusters bootstrapped with kubeadm by the ansibler. The manifests are rendered
// with helm, see the header of each file for the values used and how to regenerate it.
// They are applied in order.
var addonManifests = []string{
	"services/kuber/manifests/metrics-server.yaml",
}

// DeployAddons deploys, or updates, the cluster addons, such as metrics-server. The addons
// need a working CNI thus this task has to be scheduled after Cilium is deployed.
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
	clusterID := k8s.ClusterInfo.Id()

	if k8s.Kubeconfig == "" {
		return fmt.Errorf("cluster %s has no kubeconfig, can't deploy cluster addons", clusterID)
	}

	// The manifests may contain single quotes, which breaks [kubectl.Kubectl.KubectlApplyString],
	// thus they are copied to a cluster specific directory and applied from there.
	clusterDir := filepath.Join(OutputDir, fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)))
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		return fmt.Errorf("error while creating directory %s when deploying cluster addons: %w", clusterDir, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("Failed to remove directory %s where the addon manifests were copied", clusterDir)
		}
	}()

	k := kubectl.Kubectl{
		Kubeconfig:        k8s.Kubeconfig,
		MaxKubectlRetries: 3,
	}

	k.Stdout = comm.GetStdOut(clusterID)
	k.Stderr = comm.GetStdErr(clusterID)

	for _, manifest := range addonManifests {
		name := filepath.Base(manifest)

		content, err := os.ReadFile(manifest)
		if err != nil {
			return fmt.Errorf("error while reading addon manifest %s: %w", manifest, err)
		}

		path := filepath.Join(clusterDir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return fmt.Errorf("error while writing addon manifest to %s: %w", path, err)
		}

		logger.Debug().Msgf("Applying addon %s", name)

		if err := k.KubectlApply(path); err != nil {
			return fmt.Errorf("error while applying addon %s: %w", name, err)
		}
	}

	return nil
}
