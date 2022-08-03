// Package longhorn provides functions needed to set up the longhorn on k8s cluster
package longhorn

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

// Cluster - k8s cluster where longhorn will be set up
// Directory - directory where to create storage class manidests
type Longhorn struct {
	Cluster   *pb.K8Scluster
	Directory string
}

type zoneData struct {
	ZoneName         string
	StorageClassName string
}

const (
	longhornYaml       = "services/kuber/server/manifests/longhorn.yaml"
	storageManifestTpl = "storage-class.goyaml"
)

// SetUp function will set up the longhorn on the k8s cluster saved in l.Longhorn
func (l Longhorn) SetUp() error {
	kubectl := kubectl.Kubectl{Kubeconfig: l.Cluster.GetKubeconfig()}
	// apply longhorn.yaml
	err := kubectl.KubectlApply(longhornYaml, "")
	if err != nil {
		return fmt.Errorf("error while applying longhorn.yaml : %v", err)
	}

	//get existing sc so we can delete them if we do not need them any more
	existingSC, err := l.getStorageClasses(kubectl)
	if err != nil {
		return fmt.Errorf("error while getting existing storage classes for %s : %v", l.Cluster.ClusterInfo.Name, err)
	}
	//save "created" sc so we can find a difference and remove the old ones
	var appliedSC []string

	//load the templates
	template := utils.Templates{Directory: l.Directory}
	templateLoader := utils.TemplateLoader{Directory: utils.KuberTemplates}
	storageTpl, err := templateLoader.LoadTemplate(storageManifestTpl)
	if err != nil {
		return err
	}

	sortedNodePools := utils.GroupNodepoolsByProvider(l.Cluster.ClusterInfo)
	// get real nodes names in a case when provider appends some string to the set name
	realNodesInfo, err := kubectl.KubectlGet("nodes", "")
	if err != nil {
		return err
	}
	realNodeNames := getRealNodeNames(realNodesInfo)
	// tag nodes based on the zones
	for provider, nodepools := range sortedNodePools {
		zoneName := fmt.Sprintf("%s-zone", provider)
		storageClassName := fmt.Sprintf("longhorn-%s", zoneName)
		//flag to determine whether we need to create storage class or not
		isWorkerNodeProvider := false
		for _, nodepool := range nodepools {
			// tag worker nodes from nodepool based on the future zone
			// NOTE: the master nodes are by default set to NoSchedule, therefore we do not need to annotate them
			// If in the future, we add functionality to allow scheduling on master nodes, the longhorn will need add the annotation
			if !nodepool.IsControl {
				isWorkerNodeProvider = true
				for _, node := range nodepool.Nodes {
					// add tag to the node via kubectl annotate, use --overwrite to avoid getting error of already tagged node
					annotation := fmt.Sprintf("node.longhorn.io/default-node-tags='[\"%s\"]' --overwrite", zoneName)
					realNodeName := utils.FindName(realNodeNames, node.Name)
					err := kubectl.KubectlAnnotate("node", realNodeName, annotation)
					if err != nil {
						return fmt.Errorf("error while tagging the node %s via kubectl annotate : %v", realNodeName, err)
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
				return fmt.Errorf("error while generating %s manifest : %v", manifest, err)
			}
			//update the kubectl working directory
			kubectl.Directory = l.Directory
			// apply manifest
			err = kubectl.KubectlApply(manifest, "")
			if err != nil {
				return fmt.Errorf("error while applying %s manifest : %v", manifest, err)
			}
			appliedSC = append(appliedSC, storageClassName)
		}
	}

	err = l.deleteOldStorageClasses(existingSC, appliedSC, kubectl)
	if err != nil {
		return err
	}

	log.Info().Msgf("Longhorn successfully set-up on the %s", l.Directory)
	// Clean up
	if err := os.RemoveAll(l.Directory); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}

//getStorageClasses returns a slice of names of storage classes currently deployed in cluster
//returns slice of storage classes if successful, error otherwise
func (l *Longhorn) getStorageClasses(kc kubectl.Kubectl) (result []string, err error) {
	//define output struct
	type KubectlOutputJSON struct {
		APIVersion string                   `json:"apiVersion"`
		Items      []map[string]interface{} `json:"items"`
		Kind       string                   `json:"kind"`
		Metadata   map[string]interface{}   `json:"metadata"`
	}
	//get existing storage classes
	out, err := kc.KubectlGet("sc", "")
	if err != nil {
		return nil, err
	}
	//parse output
	var parsedJson KubectlOutputJSON
	err = json.Unmarshal(out, &parsedJson)
	if err != nil {
		return nil, err
	}
	//return name of the storage classes
	for _, sc := range parsedJson.Items {
		metadata := sc["metadata"].(map[string]interface{})
		name := metadata["name"].(string)
		result = append(result, name)
	}
	return result, nil
}

//deleteOldStorageClasses deletes storage classes, which does not have a worker nodes behind it
func (l *Longhorn) deleteOldStorageClasses(existing, applied []string, kc kubectl.Kubectl) error {
	for _, ex := range existing {
		found := false
		for _, app := range applied {
			if ex == app {
				found = true
				break
			}
		}
		if found {
			err := kc.KubectlDeleteResource("sc", ex, "")
			log.Info().Msgf("Deleting storage class %s", ex)
			if err != nil {
				return fmt.Errorf("error while deleting storage class %s due to no nodes backing it : %v", ex, err)
			}
		}
	}
	return nil
}

//getRealNodeNames will find a real node names, since some providers appends additional data after user defined name
//returns slice of a real node names
func getRealNodeNames(nodeInfo []byte) []string {
	// get slice of lines from output
	nodeInfoStrings := strings.Split(string(nodeInfo), "\n")
	// trim the column description and whitespace at the end
	nodeInfoStrings = nodeInfoStrings[1 : len(nodeInfoStrings)-1]
	var nodeNames []string
	for _, line := range nodeInfoStrings {
		fields := strings.Fields(line)
		nodeNames = append(nodeNames, fields[0])
	}
	return nodeNames
}
