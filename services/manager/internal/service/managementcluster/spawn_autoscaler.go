package managementcluster

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/service/managementcluster/internal/autoscaler"
	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func SetUpClusterAutoscaler(manifestName string, clusters *spec.Clusters) error {
	if envs.Namespace == "" {
		return nil
	}

	var (
		clusterID         = clusters.K8S.ClusterInfo.Id()
		tempClusterID     = fmt.Sprintf("%s-%s", clusterID, hash.Create(5))
		clusterDir        = filepath.Join(outputDir, tempClusterID)
		autoscalerManager = autoscaler.NewAutoscalerManager(manifestName, clusters.K8S, clusterDir)
	)

	// Create output dir
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		return fmt.Errorf("error while creating directory: %w", err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("Failed to remove directory: %s", clusterDir)
		}
	}()

	if err := autoscalerManager.SetUpClusterAutoscaler(); err != nil {
		return fmt.Errorf("error while setting up cluster autoscaler: %w", err)
	}

	return nil
}

func DriftInAutoscalerPods(logger zerolog.Logger, manifestName string, clusters *spec.Clusters) (bool, error) {
	namespace := envs.Namespace
	clusterID := clusters.K8S.ClusterInfo.Id()

	if namespace == "" {
		return false, nil
	}

	yamls, err := autoscaler.Manifests(manifestName, clusters.K8S)
	if err != nil {
		return false, err
	}

	var drift bool

	// at least 1 drift should result in the whole deployment refresh.
	for _, desired := range yamls {
		kc := kubectl.Kubectl{
			// omitting the kubeconfig will make it use the
			// managerment cluster.
			MaxKubectlRetries: -1,
			Stdout:            comm.GetStdOut(clusterID),
			Stderr:            comm.GetStdErr(clusterID),
		}

		args := []string{desired.GetName()}
		if desired.GetNamespace() != "" {
			args = append(args, "-n", desired.GetNamespace())
		}
		args = append(args, "-oyaml")

		b, err := kc.KubectlGet(desired.GetKind(), args...)
		if err != nil {
			logger.
				Err(err).
				Msg("Failed to decode autoscaler deployment in management cluster, assuming drift")
			drift = true
			continue
		}

		var (
			reader = yaml.NewYAMLToJSONDecoder(bufio.NewReader(bytes.NewReader(b)))
			live   unstructured.Unstructured
		)

		if err = reader.Decode(&live); err != nil {
			// This shouldn't error out, but in that case simply assume there is a drift.

			logger.
				Err(err).
				Msgf(
					"Failed to decode %q %q, asumming drift",
					desired.GetKind(),
					desired.GetName(),
				)

			drift = true

			// There are two cases, error is io.EOF or some unknown error
			// that occurred during parsing. For the former case continue
			// as there is no live state, for the latter there is an invalid
			// yaml, thus for both cases set the drift to true and continue
			continue
		}

		// strip the live manifest of annotations and
		// only keep the name and labels metadata which
		// is used in the autoscaler yamls.
		var (
			labels = live.GetLabels()
			name   = live.GetName()
			liveMd = map[string]any{
				"name":   name,
				"labels": labels,
			}
		)

		live.Object["metadata"] = liveMd

		if !cmp.Equal(live, desired) {
			drift = true
		}
	}

	return drift, nil
}
