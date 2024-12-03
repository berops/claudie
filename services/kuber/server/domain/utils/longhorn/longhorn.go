// Package longhorn provides functions needed to set up the longhorn on k8s cluster
package longhorn

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog/log"
)

type Longhorn struct {
	// Cluster where longhorn will be set up
	Cluster *spec.K8Scluster
	// Directory where to create storage class manifest
	Directory string
}

type zoneData struct {
	ZoneName         string
	StorageClassName string
}

type enableCA struct {
	IsAutoscaled string
}

const (
	longhornYaml         = "services/kuber/server/manifests/longhorn.yaml"
	longhornDefaultsYaml = "services/kuber/server/manifests/claudie-defaults.yaml"
	defaultSC            = "longhorn"
	storageClassLabel    = "claudie.io/storage-class"
)

// SetUp function will set up the longhorn on the k8s cluster saved in l.Longhorn
func (l *Longhorn) SetUp() error {
	k := kubectl.Kubectl{
		Kubeconfig:        l.Cluster.GetKubeconfig(),
		MaxKubectlRetries: 3,
	}
	prefix := utils.GetClusterID(l.Cluster.ClusterInfo)
	k.Stdout = comm.GetStdOut(prefix)
	k.Stderr = comm.GetStdErr(prefix)

	current, err := l.currentClaudieStorageClasses(k)
	if err != nil {
		return fmt.Errorf("error while getting existing storage classes for %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	if err := k.KubectlApply(longhornYaml); err != nil {
		return fmt.Errorf("error while applying longhorn.yaml in %s : %w", l.Directory, err)
	}

	if err := k.KubectlApply(longhornDefaultsYaml); err != nil {
		return fmt.Errorf("error while applying claudie default settings for longhorn in %s : %w", l.Directory, err)
	}

	template := templateUtils.Templates{
		Directory: l.Directory,
	}

	storageTpl, err := templateUtils.LoadTemplate(templates.StorageClassTemplate)
	if err != nil {
		return err
	}

	enableCATpl, err := templateUtils.LoadTemplate(templates.EnableClusterAutoscalerTemplate)
	if err != nil {
		return err
	}

	ca := enableCA{
		fmt.Sprintf("%v", utils.IsAutoscaled(l.Cluster)),
	}

	setting, err := template.GenerateToString(enableCATpl, ca)
	if err != nil {
		return err
	}

	if err := k.KubectlApplyString(setting); err != nil {
		return fmt.Errorf("error while applying CA setting for longhorn in cluster %s: %w", l.Cluster.ClusterInfo.Name, err)
	}

	sortedNodePools := utils.GroupNodepoolsByProviderSpecName(l.Cluster.ClusterInfo)

	var desired []string
	for provider, nodepools := range sortedNodePools {
		wk := false
		for _, np := range nodepools {
			if !np.IsControl {
				wk = true
				break
			}
		}

		if wk {
			zn := utils.SanitiseString(fmt.Sprintf("%s-zone", provider))
			sc := fmt.Sprintf("longhorn-%s", zn)
			manifest := fmt.Sprintf("%s.yaml", sc)
			data := zoneData{
				ZoneName:         zn,
				StorageClassName: sc,
			}

			if err := template.Generate(storageTpl, manifest, data); err != nil {
				return fmt.Errorf("error while generating %s manifest : %w", manifest, err)
			}

			k.Directory = l.Directory
			if err := k.KubectlApply(manifest, ""); err != nil {
				return fmt.Errorf("error while applying %s manifest : %w", manifest, err)
			}
			desired = append(desired, sc)
		}
	}

	if err := l.deleteUnused(current, desired, k); err != nil {
		return err
	}

	if err := os.RemoveAll(l.Directory); err != nil {
		return fmt.Errorf("error while deleting files %s : %w", l.Directory, err)
	}

	return nil
}

// currentClaudieStorageClasses returns a slice of names of claudie related storage classes currently deployed in cluster
func (l *Longhorn) currentClaudieStorageClasses(kc kubectl.Kubectl) (result []string, err error) {
	type KubectlOutputJSON struct {
		APIVersion string                   `json:"apiVersion"`
		Items      []map[string]interface{} `json:"items"`
		Kind       string                   `json:"kind"`
		Metadata   map[string]interface{}   `json:"metadata"`
	}

	out, err := kc.KubectlGet("sc", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("error while getting storage classes from cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	if strings.Contains(string(out), "No resources found") {
		return result, nil
	}

	var parsedJSON KubectlOutputJSON
	if err := json.Unmarshal(out, &parsedJSON); err != nil {
		return nil, fmt.Errorf("error while unmarshalling kubectl output for cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
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
func (l *Longhorn) deleteUnused(existing, applied []string, kc kubectl.Kubectl) error {
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
