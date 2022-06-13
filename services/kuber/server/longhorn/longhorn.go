// Package longhorn provides functions needed to set up the longhorn on k8s cluster
package longhorn

import (
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
	//fmt.Printf("-------%s--------\n %s \n ---------------\n", l.Cluster.ClusterInfo.Name, kubectl.Kubeconfig) //NOTE:debug print
	// apply longhorn.yaml
	err := kubectl.KubectlApply(longhornYaml, "")
	if err != nil {
		return fmt.Errorf("error while applying longhorn.yaml : %v", err)
	}

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
		for _, nodepool := range nodepools {
			// tag worker nodes from nodepool based on the future zone
			// NOTE: the master nodes are by default set to NoSchedule, therefore we do not need to annotate them
			// If in the future, we add functionality to allow scheduling on master nodes, the longhorn will need add the annotation
			if !nodepool.IsControl {
				for _, node := range nodepool.Nodes {
					// add tag to the node via kubectl annotate, use --overwrite to avoid getting error of already tagged node
					annotation := fmt.Sprintf("node.longhorn.io/default-node-tags='[\"%s\"]' --overwrite", zoneName)
					realNodeName := findName(realNodeNames, node.Name)
					err := kubectl.KubectlAnnotate("node", realNodeName, annotation)
					if err != nil {
						return fmt.Errorf("error while tagging the node %s via kubectl annotate : %v", realNodeName, err)
					}
				}
			}
		}
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
	}
	log.Info().Msgf("Longhorn successfully set-up on the %s", l.Directory)

	// Clean up
	if err := os.RemoveAll(l.Directory); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}

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

// findName will return a real node name based on the user defined one
// this is needed in case of GCP, where nodes have some info appended to their which cannot be read from terraform output
// example: gcp-control-1 -> gcp-control-1.c.project.id
func findName(realNames []string, name string) string {
	for _, n := range realNames {
		if strings.Contains(n, name) {
			return n
		}
	}
	log.Error().Msgf("Error: no real name found for %s", name)
	return ""
}
