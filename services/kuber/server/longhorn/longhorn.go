// Package longhorn provides functions needed to set up the longhorn on k8s cluster
package longhorn

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Cluster - k8s cluster where longhorn will be set up
// Directory - directory where to create storage class manifest
type Longhorn struct {
	Cluster   *pb.K8Scluster
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
	longhornEnableCaTpl  = "enable-ca.goyaml"
	storageManifestTpl   = "storage-class.goyaml"
	defaultSC            = "longhorn"
	storageClassLabel    = "claudie.io/provider-instance"
)

// SetUp function will set up the longhorn on the k8s cluster saved in l.Longhorn
func (l *Longhorn) SetUp() error {
	kubectl := kubectl.Kubectl{Kubeconfig: l.Cluster.GetKubeconfig(), MaxKubectlRetries: 3}
	// apply longhorn.yaml and settings
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", l.Cluster.ClusterInfo.Name, l.Cluster.ClusterInfo.Hash)
		kubectl.Stdout = comm.GetStdOut(prefix)
		kubectl.Stderr = comm.GetStdErr(prefix)
	}

	// Apply longhorn manifests after nodes are annotated.
	if err := l.applyManifests(kubectl); err != nil {
		return fmt.Errorf("error while applying longhorn manifests in cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	//get existing sc so we can delete them if we do not need them any more
	existingSC, err := l.getStorageClasses(kubectl)
	if err != nil {
		return fmt.Errorf("error while getting existing storage classes for %s : %w", l.Cluster.ClusterInfo.Name, err)
	}
	//save applied sc so we can find a difference with existing ones and remove the redundant ones
	var appliedSC []string

	//load the templates
	template := templateUtils.Templates{Directory: l.Directory}
	storageTpl, err := templateUtils.LoadTemplate(templates.StorageClassTemplate)
	if err != nil {
		return err
	}
	enableCATpl, err := templateUtils.LoadTemplate(templates.EnableClusterAutoscalerTemplate)
	if err != nil {
		return err
	}

	// Apply setting about CA
	enableCa := enableCA{fmt.Sprintf("%v", utils.IsAutoscaled(l.Cluster))}
	if setting, err := template.GenerateToString(enableCATpl, enableCa); err != nil {
		return err
	} else if err := kubectl.KubectlApplyString(setting); err != nil {
		return fmt.Errorf("error while applying CA setting for longhorn in cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	sortedNodePools := utils.GroupNodepoolsByProviderSpecName(l.Cluster.ClusterInfo)
	// get real nodes names in a case when provider appends some string to the set name
	realNodesInfo, err := kubectl.KubectlGetNodeNames()
	if err != nil {
		return err
	}
	realNodeNames := strings.Split(string(realNodesInfo), "\n")
	// tag nodes based on the zones
	for providerInstance, nodepools := range sortedNodePools {
		zoneName := utils.SanitiseString(fmt.Sprintf("%s-zone", providerInstance))
		storageClassName := fmt.Sprintf("longhorn-%s", zoneName)
		//flag to determine whether we need to create storage class or not
		isWorkerNodeProvider := false
		for _, np := range nodepools {
			// tag worker nodes from nodepool based on the future zone
			// NOTE: the master nodes are by default set to NoSchedule, therefore we do not need to annotate them
			// If in the future, we add functionality to allow scheduling on master nodes, the longhorn will need add the annotation
			if !np.IsControl {
				isWorkerNodeProvider = true
				for _, node := range np.GetNodes() {
					annotation := fmt.Sprintf("node.longhorn.io/default-node-tags='[\"%s\"]'", zoneName)
					realNodeName := utils.FindName(realNodeNames, node.Name)
					if realNodeName == "" {
						log.Warn().Str("cluster", utils.GetClusterID(l.Cluster.ClusterInfo)).Msgf("Node %s was not found in cluster %v", node.Name, realNodeNames)
						continue
					}
					// Add tag to the node via kubectl annotate, use --overwrite to avoid getting error of already tagged node
					if err := kubectl.KubectlAnnotate("node", realNodeName, annotation, "--overwrite"); err != nil {
						return fmt.Errorf("error while annotating the node %s from cluster %s via kubectl annotate : %w", realNodeName, l.Cluster.ClusterInfo.Name, err)
					}
				}
			}
		}
		if isWorkerNodeProvider {
			// create storage class manifest based on zones from templates
			zoneData := zoneData{ZoneName: zoneName, StorageClassName: storageClassName}
			manifest := fmt.Sprintf("%s.yaml", storageClassName)
			err := template.Generate(storageTpl, manifest, zoneData)
			if err != nil {
				return fmt.Errorf("error while generating %s manifest : %w", manifest, err)
			}
			//update the kubectl working directory
			kubectl.Directory = l.Directory
			// apply manifest
			err = kubectl.KubectlApply(manifest, "")
			if err != nil {
				return fmt.Errorf("error while applying %s manifest : %w", manifest, err)
			}
			appliedSC = append(appliedSC, storageClassName)
		}
	}

	err = l.deleteOldStorageClasses(existingSC, appliedSC, kubectl)
	if err != nil {
		return err
	}

	// Clean up
	if err := os.RemoveAll(l.Directory); err != nil {
		return fmt.Errorf("error while deleting files %s : %w", l.Directory, err)
	}
	return nil
}

// getStorageClasses returns a slice of names of storage classes currently deployed in cluster
// returns slice of storage classes if successful, error otherwise
func (l *Longhorn) getStorageClasses(kc kubectl.Kubectl) (result []string, err error) {
	//define output struct
	type KubectlOutputJSON struct {
		APIVersion string                   `json:"apiVersion"`
		Items      []map[string]interface{} `json:"items"`
		Kind       string                   `json:"kind"`
		Metadata   map[string]interface{}   `json:"metadata"`
	}
	//get existing storage classes
	out, err := kc.KubectlGet("sc", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("error while getting storage classes from cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}
	//no storage class defined yet
	if strings.Contains(string(out), "No resources found") {
		return result, nil
	}
	//parse output
	var parsedJSON KubectlOutputJSON
	err = json.Unmarshal(out, &parsedJSON)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling kubectl output for cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}
	//return name of the storage classes
	for _, sc := range parsedJSON.Items {
		metadata := sc["metadata"].(map[string]interface{})
		name := metadata["name"].(string)
		//check if storage class has a claudie label
		if labels, ok := metadata["labels"]; ok {
			labelsMap := labels.(map[string]interface{})
			if _, ok := labelsMap[storageClassLabel]; ok {
				result = append(result, name)
			}
		}
	}
	return result, nil
}

// deleteOldStorageClasses deletes storage classes, which does not have a worker nodes behind it
func (l *Longhorn) deleteOldStorageClasses(existing, applied []string, kc kubectl.Kubectl) error {
	for _, ex := range existing {
		if ex == defaultSC {
			//ignore the default sc
			continue
		}
		found := false
		for _, app := range applied {
			if ex == app {
				found = true
				break
			}
		}
		//if not found in applied, delete the sc
		if !found {
			err := kc.KubectlDeleteResource("sc", ex)
			log.Debug().Msgf("Deleting storage class %s", ex)
			if err != nil {
				return fmt.Errorf("error while deleting storage class %s due to no nodes backing it : %w", ex, err)
			}
		}
	}
	return nil
}

func (l *Longhorn) applyManifests(kc kubectl.Kubectl) error {
	// Apply longhorn.yaml
	if err := kc.KubectlApply(longhornYaml); err != nil {
		return fmt.Errorf("error while applying longhorn.yaml in %s : %w", l.Directory, err)
	}
	// Apply longhorn setting
	if err := kc.KubectlApply(longhornDefaultsYaml, ""); err != nil {
		return fmt.Errorf("error while applying settings for longhorn in %s : %w", l.Directory, err)
	}
	return nil
}
