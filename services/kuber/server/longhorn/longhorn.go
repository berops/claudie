// Package longhorn provides functions needed to set up the longhorn on k8s cluster
package longhorn

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/Berops/platform/utils"
)

// Cluster - k8s clusther where longhorn will be set up
type Longhorn struct {
	Cluster *pb.K8Scluster
}

type nodeData struct {
	NodeName string
	ZoneName string
}

type zoneData struct {
	ZoneName         string
	StorageClassName string
}

const (
	longhornYaml       = "services/kuber/server/manifests/longhorn.yaml"
	outputDir          = "services/kuber/server/clusters"
	nodeManifestTpl    = "cluster-machine.goyaml"
	storageManifestTpl = "storage-class.goyaml"
)

// SetUp function will set up the longhorn on the k8s cluster saved in l.Longhorn
func (l Longhorn) SetUp() error {
	clusterID := fmt.Sprintf("%s-%s", l.Cluster.ClusterInfo.Name, l.Cluster.ClusterInfo.Hash)
	clusterDir := filepath.Join(outputDir, clusterID)
	kubectl := kubectl.Kubectl{Kubeconfig: l.Cluster.GetKubeconfig()}

	// apply longhorn.yaml
	err := kubectl.KubectlApply(longhornYaml, "")
	if err != nil {
		return fmt.Errorf("error while applying longhorn.yaml : %v", err)
	}

	//load the templates
	template := utils.Templates{Directory: clusterDir}
	templateLoader := utils.TemplateLoader{Directory: utils.KuberTemplates}
	storageTpl, err := templateLoader.LoadTemplate(storageManifestTpl)
	if err != nil {
		return err
	}

	// apply cluster-machine.yaml based on zones
	sortedNodePools := utils.GroupNodepoolsByProvider(l.Cluster.ClusterInfo)
	for provider, nodepools := range sortedNodePools {
		zoneName := fmt.Sprintf("%s-zone", provider)
		storageClassName := fmt.Sprintf("longhorn-%s", zoneName)
		for _, nodepool := range nodepools {
			//tag nodes from nodepool based on the future zone
			for _, node := range nodepool.Nodes {
				// add tag to the node via kubectl annotate
				annotation := fmt.Sprintf("node.longhorn.io/default-node-tags='[\"%s\"]'", zoneName)
				err = kubectl.KubectlAnnotate("node", node.Name, annotation)
				if err != nil {
					return fmt.Errorf("error while tagging the node %s via kubectl annotate : %v", node.Name, err)
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
		kubectl.Directory = clusterDir
		// apply manifest
		err = kubectl.KubectlApply(manifest, "")
		if err != nil {
			return fmt.Errorf("error while applying %s manifest : %v", manifest, err)
		}
	}
	fmt.Println("---------------")
	fmt.Println(kubectl.Kubeconfig)
	fmt.Println("---------------")
	// Clean up
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}
