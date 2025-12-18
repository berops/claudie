package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/sanitise"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	defaultSC         = "longhorn"
	storageClassLabel = "claudie.io/storage-class"
)

type zoneData struct {
	ZoneName         string
	StorageClassName string
}

func ReconcileLonghornStorageClasses(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Reconciling longhorn storage classes")

	var k8s *spec.K8SclusterV2

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = do.Create.K8S
	case *spec.TaskV2_Update:
		k8s = do.Update.State.K8S
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to setup storage, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var (
		tempClusterId = fmt.Sprintf("%s-%s", k8s.ClusterInfo.Id(), hash.Create(7))
		clusterDir    = filepath.Join(OutputDir, tempClusterId)
		template      = templateUtils.Templates{
			Directory: clusterDir,
		}
		k = kubectl.Kubectl{
			Kubeconfig:        k8s.Kubeconfig,
			MaxKubectlRetries: 3,
			Directory:         clusterDir,
			Stdout:            comm.GetStdOut(k8s.ClusterInfo.Id()),
			Stderr:            comm.GetStdErr(k8s.ClusterInfo.Id()),
		}
	)

	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		logger.Err(err).Msg("Failed to create directory for templates")
		tracker.Diagnostics.Push(err)
		return
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			logger.Err(err).Msg("Failed to remove directory where templates were generated")
			return
		}
	}()

	storageTpl, err := templateUtils.LoadTemplate(templates.StorageClassTemplate)
	if err != nil {
		logger.Err(err).Msg("Failed to load claudie storage class template")
		tracker.Diagnostics.Push(err)
		return
	}

	current, err := currentClaudieStorageClasses(k)
	if err != nil {
		logger.Err(err).Msg("Failed to retrieve existing claudie storage classes for longhorn")
		tracker.Diagnostics.Push(err)
		return
	}

	var desired []string
	for provider, nps := range nodepools.ByProviderSpecName(k8s.ClusterInfo.NodePools) {
		wk := false
		for _, np := range nps {
			if !np.IsControl {
				wk = true
				break
			}
		}

		if wk {
			zn := sanitise.String(fmt.Sprintf("%s-zone", provider))
			sc := fmt.Sprintf("longhorn-%s", zn)
			manifest := fmt.Sprintf("%s.yaml", sc)
			data := zoneData{
				ZoneName:         zn,
				StorageClassName: sc,
			}

			if err := template.Generate(storageTpl, manifest, data); err != nil {
				logger.Err(err).Msg("Failed to generate claudie storage class template")
				tracker.Diagnostics.Push(err)
				return
			}

			if err := k.KubectlApply(manifest, ""); err != nil {
				logger.Err(err).Msg("Failed to apply claudie storage class template")
				tracker.Diagnostics.Push(err)
				return
			}
			desired = append(desired, sc)
		}
	}

	if err := deleteUnused(current, desired, k); err != nil {
		logger.Err(err).Msg("Failed to reconile claudie storage class templates")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Successfully reconciled longhorn storage classes")

}

// currentClaudieStorageClasses returns a slice of names of claudie related storage classes currently deployed in cluster
func currentClaudieStorageClasses(kc kubectl.Kubectl) (result []string, err error) {
	type KubectlOutputJSON struct {
		APIVersion string                   `json:"apiVersion"`
		Items      []map[string]interface{} `json:"items"`
		Kind       string                   `json:"kind"`
		Metadata   map[string]interface{}   `json:"metadata"`
	}

	out, err := kc.KubectlGet("sc", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("error while getting storage classes: %w", err)
	}

	if strings.Contains(string(out), "No resources found") {
		return result, nil
	}

	var parsedJSON KubectlOutputJSON
	if err := json.Unmarshal(out, &parsedJSON); err != nil {
		return nil, fmt.Errorf("error while unmarshalling kubectl output %w", err)
	}

	for _, sc := range parsedJSON.Items {
		metadata := sc["metadata"].(map[string]interface{})
		name := metadata["name"].(string)

		if labels, ok := metadata["labels"]; ok {
			labelsMap := labels.(map[string]interface{})
			if _, ok := labelsMap[storageClassLabel]; ok {
				result = append(result, name)
			}
		}
	}

	return result, nil
}

// deleteUnused deleted unused storage classes previously created by claudie.
func deleteUnused(existing, applied []string, kc kubectl.Kubectl) error {
	for _, ex := range existing {
		if ex == defaultSC {
			//ignore the default sc
			continue
		}
		if !slices.Contains(applied, ex) {
			log.Debug().Msgf("Deleting storage class %s", ex)
			if err := kc.KubectlDeleteResource("sc", ex); err != nil {
				return fmt.Errorf("error while deleting storage class %s due to no nodes backing it : %w", ex, err)
			}
		}
	}
	return nil
}
