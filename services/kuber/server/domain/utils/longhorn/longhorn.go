// Package longhorn provides functions needed to set up the longhorn on k8s cluster
package longhorn

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/sanitise"
	"github.com/berops/claudie/internal/templateUtils"
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

	// minVersionForDrainPolicy is the minimum Longhorn version that supports
	// the node-drain-policy value "block-for-eviction-if-contains-last-replica"
	minVersionForDrainPolicy = "v1.9.2"
	// nodeDrainPolicyValue is the value to set for node-drain-policy setting
	nodeDrainPolicyValue = "block-for-eviction-if-contains-last-replica"
)

// SetUp function will set up the longhorn on the k8s cluster saved in l.Longhorn
func (l *Longhorn) SetUp() error {
	k := kubectl.Kubectl{
		Kubeconfig:        l.Cluster.GetKubeconfig(),
		MaxKubectlRetries: 3,
	}
	k.Stdout = comm.GetStdOut(l.Cluster.ClusterInfo.Id())
	k.Stderr = comm.GetStdErr(l.Cluster.ClusterInfo.Id())
	log := loggerutils.WithClusterName(l.Cluster.ClusterInfo.Id())

	current, err := l.currentClaudieStorageClasses(k)
	if err != nil {
		return fmt.Errorf("error while getting existing storage classes for %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	// Check if Longhorn is already installed and if version is below v1.9.2
	// If so, patch the node-drain-policy setting before upgrading
	currentVersion, err := l.getCurrentLonghornVersion(k)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get current Longhorn version, proceeding with installation")
	} else if currentVersion != "" {
		isBelow, err := isVersionBelow(currentVersion, minVersionForDrainPolicy)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to compare Longhorn versions, proceeding with installation")
		} else if isBelow {
			log.Info().Msgf("Current Longhorn version %s is below %s, patching node-drain-policy setting", currentVersion, minVersionForDrainPolicy)
			if err := l.patchNodeDrainPolicy(k, nodeDrainPolicyValue); err != nil {
				log.Warn().Err(err).Msg("Failed to patch node-drain-policy setting, proceeding with installation")
			}
		}
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
		fmt.Sprintf("%v", l.Cluster.AnyAutoscaledNodePools()),
	}

	setting, err := template.GenerateToString(enableCATpl, ca)
	if err != nil {
		return err
	}

	if err := k.KubectlApplyString(setting); err != nil {
		return fmt.Errorf("error while applying CA setting for longhorn in cluster %s: %w", l.Cluster.ClusterInfo.Name, err)
	}

	var desired []string
	for provider, nps := range nodepools.ByProviderSpecName(l.Cluster.ClusterInfo.NodePools) {
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

// getCurrentLonghornVersion retrieves the currently installed Longhorn version from the cluster.
// Returns empty string if Longhorn is not installed or version cannot be determined.
func (l *Longhorn) getCurrentLonghornVersion(kc kubectl.Kubectl) (string, error) {
	type deploymentJSON struct {
		Items []struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"items"`
	}

	out, err := kc.KubectlGet("deployment", "-n", "longhorn-system", "-l", "app.kubernetes.io/name=longhorn", "-o", "json")
	if err != nil {
		return "", err
	}

	if strings.Contains(string(out), "No resources found") {
		return "", nil
	}

	var parsed deploymentJSON
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", fmt.Errorf("error unmarshalling longhorn deployment: %w", err)
	}

	if len(parsed.Items) == 0 {
		return "", nil
	}

	version, ok := parsed.Items[0].Metadata.Labels["app.kubernetes.io/version"]
	if !ok {
		return "", nil
	}

	return version, nil
}

// isVersionBelow checks if the given version is below the target version using semver comparison.
// Returns true if version < target, false otherwise.
func isVersionBelow(version, target string) (bool, error) {
	// Handle empty version (Longhorn not installed)
	if version == "" {
		return false, nil
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return false, fmt.Errorf("error parsing version %s: %w", version, err)
	}

	t, err := semver.NewVersion(target)
	if err != nil {
		return false, fmt.Errorf("error parsing target version %s: %w", target, err)
	}

	return v.LessThan(t), nil
}

// patchNodeDrainPolicy patches the node-drain-policy setting to the specified value.
// This is needed for upgrades from Longhorn versions below v1.9.2.
func (l *Longhorn) patchNodeDrainPolicy(kc kubectl.Kubectl, value string) error {
	patchData := fmt.Sprintf(`{"value":"%s"}`, value)
	if err := kc.KubectlPatch("setting.longhorn.io", "node-drain-policy", patchData, "-n", "longhorn-system", "--type=merge"); err != nil {
		return fmt.Errorf("error patching node-drain-policy setting: %w", err)
	}
	log.Info().Msgf("Patched node-drain-policy setting to %s", value)
	return nil
}
