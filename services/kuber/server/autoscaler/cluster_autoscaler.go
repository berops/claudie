package autoscaler

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"text/template"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	clusterAutoscalerTemplate   = "cluster-autoscaler.goyaml"
	clusterAutoscalerDeployment = "ca.yaml"
	defaultAdapterPort          = "50000"
)

type AutoscalerBuilder struct {
	projectName string
	cluster     *pb.K8Scluster
	directory   string
}

type AutoscalerDeploymentData struct {
	ClusterID         string
	AdapterPort       string
	ClusterName       string
	ProjectName       string
	KubernetesVersion string
}

// NewAutoscalerBuilder returns configured AutoscalerBuilder which can set up or remove Cluster Autoscaler.
func NewAutoscalerBuilder(projectName string, cluster *pb.K8Scluster, directory string) *AutoscalerBuilder {
	return &AutoscalerBuilder{projectName: projectName, cluster: cluster, directory: directory}
}

// SetUpClusterAutoscaler deploys all resources necessary to set up a Cluster Autoscaler.
func (ab *AutoscalerBuilder) SetUpClusterAutoscaler() error {
	// Create files from templates.
	if err := ab.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{Directory: ab.directory, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", ab.cluster.ClusterInfo.Name, ab.cluster.ClusterInfo.Hash)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	if err := kc.KubectlApply(clusterAutoscalerDeployment, "-n", envs.Namespace); err != nil {
		return fmt.Errorf("error while applying cluster autoscaler for cluster %s : %w", ab.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(ab.directory)
}

// DestroyClusterAutoscaler removes all resources regarding Cluster Autoscaler.
func (ab *AutoscalerBuilder) DestroyClusterAutoscaler() error {
	// Create files from templates.
	if err := ab.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{Directory: ab.directory, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", ab.cluster.ClusterInfo.Name, ab.cluster.ClusterInfo.Hash)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	if err := kc.KubectlDeleteManifest(clusterAutoscalerDeployment, "-n", envs.Namespace); err != nil {
		return fmt.Errorf("error while deleting cluster autoscaler for cluster %s : %w", ab.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(ab.directory)
}

// generateFiles generates all manifests required for deploying Cluster Autoscaler.
func (ab *AutoscalerBuilder) generateFiles() error {
	tpl := templateUtils.Templates{Directory: ab.directory}
	var ca *template.Template
	var err error

	// Load templates
	if ca, err = templateUtils.LoadTemplate(templates.ClusterAutoscalerTemplate); err != nil {
		return fmt.Errorf("error loading cluster autoscaler template : %w", err)
	}
	// Prepare data
	clusterId := fmt.Sprintf("%s-%s", ab.cluster.ClusterInfo.Name, ab.cluster.ClusterInfo.Hash)
	version, err := getK8sVersion(ab.cluster.Kubernetes)
	if err != nil {
		return err
	}

	caData := &AutoscalerDeploymentData{
		ClusterName:       ab.cluster.ClusterInfo.Name,
		ProjectName:       ab.projectName,
		ClusterID:         clusterId,
		AdapterPort:       defaultAdapterPort,
		KubernetesVersion: version,
	}

	if err := tpl.Generate(ca, clusterAutoscalerDeployment, caData); err != nil {
		return fmt.Errorf("error generating cluster autoscaler deployment : %w", err)
	}

	return nil
}

// getK8sVersion returns a simplified semver version of the cluster. The same version is then used for
// Cluster Autoscaler image, as per project recommendation.
func getK8sVersion(version string) (string, error) {
	// Regular expression pattern to match semver format
	pattern := `(\d+)\.(\d+)\.(\d+)`
	// Find the first occurrence of semver format in the input string
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(version)

	if len(match) >= 4 {
		// Verify version is higher than v1.25.0 as external gRPC provider is not supported in older versions
		// TODO: remove this condition once v1.24.X will not be supported
		minor, err := strconv.Atoi(match[2])
		if err != nil {
			return "", fmt.Errorf("failed to verify kubernetes version vX.%s.Y : %w", match[2], err)
		}
		if minor < 25 {
			return "v1.25.0", nil
		}
		return fmt.Sprintf("v%s.%s.%s", match[1], match[2], match[3]), nil
	}
	return "", fmt.Errorf("failed to parse %s into autoscaler image tag [vX.Y.Z]", version)
}
