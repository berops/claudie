package longhorn

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	kuberGoYAMLTemplates "github.com/berops/claudie/services/kuber/templates"
)

const (
	// longhorn.yaml is the file responsible for deploying Longhorn to the cluster
	longhornYAMLFilePath = "services/kuber/server/manifests/longhorn.yaml"

	// claudie-defaults.yaml file is responsible for further configuring Longhorn
	// according to Claudie standards.
	longhornDefaultSettingsYAMLFilePath = "services/kuber/server/manifests/claudie-defaults.yaml"

	// If a storageclass has this label, then it signifies that the storageclass
	// is being managed by Claudie.
	claudieLabel = "claudie.io/provider-instance"

	defaultStorageclassName = "longhorn"
)

type (
	zoneData struct {
		ZoneName         string
		StorageClassName string
	}

	enableCA struct {
		IsAutoscaled string
	}

	Longhorn struct {
		// Target K8s cluster where longhorn will be set up
		Cluster *pb.K8Scluster

		// Output directory where storageclass manifest will be created.
		OutputDirectory string
	}
)

// SetUp function will deploy and configure Longhorn on the target K8s cluster
// (represented by l.Cluster).
// NOTE - CA stands for Cluster Autoscaler.
func (l *Longhorn) SetUp() error {
	kubectl := kubectl.Kubectl{
		Kubeconfig:        l.Cluster.GetKubeconfig(),
		MaxKubectlRetries: 3,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", l.Cluster.ClusterInfo.Name, l.Cluster.ClusterInfo.Hash)

		kubectl.Stdout = comm.GetStdOut(prefix)
		kubectl.Stderr = comm.GetStdErr(prefix)
	}

	// Apply Longhorn deployment and configuration files to the cluster.
	// The K8s cluster nodes should be annotated at this point so that the compute nodes
	// are identifiable. Claudie will deploy Longhorn to compute nodes only.
	if err := l.applyManifests(kubectl); err != nil {
		return fmt.Errorf("error while applying Longhorn manifests to cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	existingStorageclassNames, err := l.getExistingStorageclassNames(kubectl)
	if err != nil {
		return fmt.Errorf("error while getting existing storage classes for %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	// Those storageclasses which are under use by any worker node,
	// we will call them active storageclasses.
	var activeStorageclassNames []string

	// Load the templates
	templates := templateUtils.Templates{Directory: l.OutputDirectory}
	template, err := templateUtils.LoadTemplate(kuberGoYAMLTemplates.StorageClassTemplate)
	if err != nil {
		return err
	}
	enableCATemplate, err := templateUtils.LoadTemplate(kuberGoYAMLTemplates.EnableClusterAutoscalerTemplate)
	if err != nil {
		return err
	}

	// Apply setting about Cluster Autoscaler
	enableCA := enableCA{fmt.Sprintf("%v", utils.IsAutoscaled(l.Cluster))}
	if setting, err := templates.GenerateToString(enableCATemplate, enableCA); err != nil {
		return err
	} else if err := kubectl.KubectlApplyString(setting); err != nil {
		return fmt.Errorf("error while applying CA setting for longhorn in cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	sortedNodePools := utils.GroupNodepoolsByProviderSpecName(l.Cluster.ClusterInfo)
	// get node names in a case when provider appends some string to the set name
	realNodesInfo, err := kubectl.KubectlGetNodeNames()
	if err != nil {
		return err
	}
	realNodeNames := strings.Split(string(realNodesInfo), "\n")
	// tag nodes based on the zones
	for providerInstance, nodepools := range sortedNodePools {
		zoneName := fmt.Sprintf("%s-zone", providerInstance)
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
					annotation := fmt.Sprintf("node.longhorn.io/default-node-tags='[\"%s\"]'", zoneName)
					realNodeName := utils.FindName(realNodeNames, node.Name)
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
			err := templates.Generate(template, manifest, zoneData)
			if err != nil {
				return fmt.Errorf("error while generating %s manifest : %w", manifest, err)
			}
			//update the kubectl working directory
			kubectl.Directory = l.OutputDirectory
			// apply manifest
			err = kubectl.KubectlApply(manifest, "")
			if err != nil {
				return fmt.Errorf("error while applying %s manifest : %w", manifest, err)
			}
			activeStorageclassNames = append(activeStorageclassNames, storageClassName)
		}
	}

	if err = l.deleteUnusedStoragelasses(existingStorageclassNames, activeStorageclassNames, kubectl); err != nil {
		return err
	}

	// Perform cleanup by deleting the manifest files which were generated in l.OutputDirectory.
	if err := os.RemoveAll(l.OutputDirectory); err != nil {
		return fmt.Errorf("error while deleting files from %s : %w", l.OutputDirectory, err)
	}

	return nil
}

// getExistingStorageclassNames returns a slice of names of storageclasses currently deployed
// in the cluster. Returns slice of storageclasses if successful, error otherwise.
func (l *Longhorn) getExistingStorageclassNames(kc kubectl.Kubectl) (existingStorageclassNames []string, err error) {
	// Get existing storageclasses
	output, err := kc.KubectlGet("sc", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("error while getting existing storageclasses from cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	} else if strings.Contains(string(output), "No resources found") {
		// No storageclasses defined yet.
		return existingStorageclassNames, nil
	}

	type Output struct {
		APIVersion string                 `json:"apiVersion"`
		Kind       string                 `json:"kind"`
		Metadata   map[string]interface{} `json:"metadata"`

		Items []map[string]interface{} `json:"items"`
	}
	// Parse the JSON output to the Output struct.
	var parsedOutput Output
	if err = json.Unmarshal(output, &parsedOutput); err != nil {
		return nil, fmt.Errorf("error while unmarshalling kubectl output for cluster %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	// Construct the slice of names of existing storageclasses.
	for _, storageClass := range parsedOutput.Items {
		metadata := storageClass["metadata"].(map[string]interface{})
		name := metadata["name"].(string)

		// Verify that the storageclass is managed by Claudie,
		// by checking whether it has the Claudie label.
		if labels, ok := metadata["labels"]; ok {
			labels := labels.(map[string]interface{})
			if _, ok := labels[claudieLabel]; ok {
				existingStorageclassNames = append(existingStorageclassNames, name)
			}
		}
	}

	return existingStorageclassNames, nil
}

// deleteOldStorageclasses deletes storageclasses, which are not being used
// by any compute nodes.
func (l *Longhorn) deleteUnusedStoragelasses(existingStorageclassNames, activeStorageclassNames []string, kubectl kubectl.Kubectl) error {
	for _, existingStorageclassName := range existingStorageclassNames {
		// Ignore the default storageclass.
		if existingStorageclassName == defaultStorageclassName {
			continue
		}

		isActive := false
		// If the exitsing storageclass is active, then nothing to do.
		for _, activeStorageclassName := range activeStorageclassNames {
			if existingStorageclassName == activeStorageclassName {
				isActive = true

				break
			}
		}

		// But if the existing storageclass is unused, then delete it.
		if !isActive {
			log.Debug().Msgf("Deleting storage class %s", existingStorageclassName)

			if err := kubectl.KubectlDeleteResource("sc", existingStorageclassName); err != nil {
				// TODO: understand what is meant by 'due to no nodes backing it'.
				return fmt.Errorf("error while deleting storageclass %s due to no nodes backing it : %w", existingStorageclassName, err)
			}
		}
	}

	return nil
}

func (l *Longhorn) applyManifests(kubectl kubectl.Kubectl) error {
	// Apply longhorn.yaml
	if err := kubectl.KubectlApply(longhornYAMLFilePath); err != nil {
		return fmt.Errorf("error while applying longhorn.yaml to %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	// Apply longhorn settings
	if err := kubectl.KubectlApply(longhornDefaultSettingsYAMLFilePath, ""); err != nil {
		return fmt.Errorf("error while applying settings for Longhorn to %s : %w", l.Cluster.ClusterInfo.Name, err)
	}

	return nil
}
