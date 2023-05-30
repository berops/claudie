package clusterAutoscaler

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
	kuberGoYAMLTemplates "github.com/berops/claudie/services/kuber/templates"
)

const (
	clusterAutoscalerTemplateFileName  = "cluster-autoscaler.goyaml"
	clusterAutoscalerDeloymentFileName = "ca.yaml"

	// TODO: add a helpful comment which explains what this is for.
	defaultAdapterPort = "50000"
)

type TemplateParameters struct {
	ProjectName       string
	ClusterID         string
	ClusterName       string
	AdapterPort       string
	KubernetesVersion string
}

type ClusterAutoscalerManager struct {
	projectName string
	cluster     *pb.K8Scluster

	outputDirectory string
}

// NewAutoscalerBuilder returns a configured instance of ClusterAutoscalerManager,
// which can set up or remove Cluster Autoscaler for the given cluster.
func NewAutoscalerBuilder(projectName string, cluster *pb.K8Scluster, outputDirectory string) *ClusterAutoscalerManager {
	return &ClusterAutoscalerManager{
		projectName: projectName,
		cluster:     cluster,

		outputDirectory: outputDirectory,
	}
}

// SetupClusterAutoscaler deploys all resources necessary to set up a Cluster Autoscaler for the
// given K8s cluster.
func (c *ClusterAutoscalerManager) SetupClusterAutoscaler() error {
	// Generate from templates, deployment file for cluster-autoscaler and autoscaler-adapter
	// and configmap for cluster-autoscaler.
	if err := c.generateFiles(); err != nil {
		return err
	}

	kc := kubectl.Kubectl{Directory: c.outputDirectory, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", c.cluster.ClusterInfo.Name, c.cluster.ClusterInfo.Hash)

		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	// Apply generated files to the given K8s cluster.
	if err := kc.KubectlApply(clusterAutoscalerDeloymentFileName, "-n", envs.Namespace); err != nil {
		return fmt.Errorf("error while applying cluster autoscaler for cluster %s : %w", c.cluster.ClusterInfo.Name, err)
	}

	// Cleanup - remove generated files
	return os.RemoveAll(c.outputDirectory)
}

// DestroyClusterAutoscaler removes all resources related Cluster Autoscaler
// from the given K8s cluster.
func (c *ClusterAutoscalerManager) DestroyClusterAutoscaler() error {
	// Create files from templates.
	if err := c.generateFiles(); err != nil {
		return err
	}

	kc := kubectl.Kubectl{Directory: c.outputDirectory, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", c.cluster.ClusterInfo.Name, c.cluster.ClusterInfo.Hash)

		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	// Deleting resources from the given K8s cluster (taking help of the generated files).
	if err := kc.KubectlDeleteManifest(clusterAutoscalerDeloymentFileName, "-n", envs.Namespace); err != nil {
		return fmt.Errorf("error while deleting cluster autoscaler for cluster %s : %w", c.cluster.ClusterInfo.Name, err)
	}

	// Cleanup - remove generated files
	return os.RemoveAll(c.outputDirectory)
}

// generateFiles generates all manifests required for deploying Cluster Autoscaler.
func (c *ClusterAutoscalerManager) generateFiles() error {
	// Construct TemplateParameters
	var (
		clusterId       = fmt.Sprintf("%s-%s", c.cluster.ClusterInfo.Name, c.cluster.ClusterInfo.Hash)
		k8sVersion, err = getK8sVersion(c.cluster.Kubernetes)
	)
	if err != nil {
		return err
	}
	templateParameters := &TemplateParameters{
		ClusterName:       c.cluster.ClusterInfo.Name,
		ProjectName:       c.projectName,
		ClusterID:         clusterId,
		AdapterPort:       defaultAdapterPort,
		KubernetesVersion: k8sVersion,
	}

	// Generate file from the template.
	templates := templateUtils.Templates{Directory: c.outputDirectory}
	template, err := templateUtils.LoadTemplate(kuberGoYAMLTemplates.ClusterAutoscalerTemplate)
	if err != nil {
		return fmt.Errorf("error loading cluster autoscaler template : %w", err)
	}
	if err := templates.Generate(template, clusterAutoscalerDeloymentFileName, templateParameters); err != nil {
		return fmt.Errorf("error generating cluster autoscaler deployment : %w", err)
	}

	return nil
}

// getK8sVersion returns a simplified semver version of the cluster.
// The same version is then used for Cluster Autoscaler image (as recommended in the Cluster
// Autoscaler documentation).
func getK8sVersion(version string) (string, error) {
	// Regular expression pattern to match semver format
	pattern := `(\d+)\.(\d+)\.(\d+)`
	// Find the first occurrence of semver format in the input string
	re := regexp.MustCompile(pattern)

	match := re.FindStringSubmatch(version)
	if len(match) >= 4 {
		minor, err := strconv.Atoi(match[2])
		if err != nil {
			return "", fmt.Errorf("failed to verify kubernetes version vX.%s.Y : %w", match[2], err)

			// Verify version is higher than 1.25.0.
			// (external gRPC provider is not supported in older versions of Cluster Autoscaler).
			// TODO: remove this condition once Kubernetes version v1.24.X will not be supported by Claudie.
		} else if minor < 25 {
			return "1.25.0", nil
		}

		return fmt.Sprintf("v%s.%s.%s", match[1], match[2], match[3]), nil
	}
	return "", fmt.Errorf("failed to parse %s into autoscaler image tag [vX.Y.Z]", version)
}
