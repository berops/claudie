package longhorn

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/Berops/platform/utils"
)

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
	longhornYaml       = "../manifests/longhorn.yaml"
	nodeManifestTpl    = "cluster-machine.goyaml"
	storageManifestTpl = "storage-class.goyaml"
)

func (l Longhorn) SetUp() error {
	kubectl := kubectl.Kubectl{Kubeconfig: l.Cluster.Kubeconfig}
	clusterID := fmt.Sprintf("%s-%s", l.Cluster.ClusterInfo.Name, l.Cluster.ClusterInfo.Hash)

	// apply longhorn.yaml
	err := kubectl.KubectlApply(longhornYaml, "")
	if err != nil {
		return fmt.Errorf("error while applying longhorn.yaml : %v", err)
	}

	// apply cluster-machine.yaml based on zones
	sortedNodePools := utils.GroupNodepoolsByProvider(l.Cluster.ClusterInfo)
	template := utils.Templates{Directory: clusterID}
	templateLoader := utils.TemplateLoader{Directory: utils.KuberTemplates}
	nodeTpl, err := templateLoader.LoadTemplate(nodeManifestTpl)
	if err != nil {
		return err
	}
	storageTpl, err := templateLoader.LoadTemplate(storageManifestTpl)
	if err != nil {
		return err
	}
	for provider, nodepools := range sortedNodePools {
		zoneName := fmt.Sprintf("%s-zone", provider)
		storageClassName := fmt.Sprintf("longhorn-%s", zoneName)
		for _, nodepool := range nodepools {
			//tag nodes from nodepool based on the future zone
			for _, node := range nodepool.Nodes {
				//generate manifest
				nodeData := nodeData{NodeName: node.Name, ZoneName: zoneName}
				manifest := fmt.Sprintf("%s.yaml", node.Name)
				err := template.Generate(nodeTpl, manifest, nodeData)
				if err != nil {
					return fmt.Errorf("error while generating %s manifest : %v", manifest, err)
				}
				// apply manifest
				err = kubectl.KubectlApply(manifest, "")
				if err != nil {
					return fmt.Errorf("error while applying %s manifest : %v", manifest, err)
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
		// apply manifest
		err = kubectl.KubectlApply(manifest, "")
		if err != nil {
			return fmt.Errorf("error while applying %s manifest : %v", manifest, err)
		}
	}
	return nil
}
