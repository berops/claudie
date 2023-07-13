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
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// Deployment file name.
	clusterAutoscalerDeployment = "ca.yaml"
	// Default port for Claudie autoscaler-adapter.
	defaultAdapterPort = "50000"
	// Default hostname for Claudie operator.
	defaultOperatorHostname = "claudie-operator"
	// Default port for Claudie operator.
	defaultOperatorPort = "50058"
)

// ClusterAutoscalerManager either creates or destroys Cluster Autoscaler resources for given k8s cluster.
type AutoscalerManager struct {
	// Project name where k8s cluster is defined.
	projectName string
	// K8s cluster.
	cluster *pb.K8Scluster
	// Output directory.
	directory string
}

type autoscalerDeploymentData struct {
	ClusterID         string
	AdapterPort       string
	ClusterName       string
	ProjectName       string
	KubernetesVersion string
	OperatorHostname  string
	OperatorPort      string
}

// NewAutoscalerManager returns configured AutoscalerManager which can set up or remove Cluster Autoscaler.
func NewAutoscalerManager(projectName string, cluster *pb.K8Scluster, directory string) *AutoscalerManager {
	return &AutoscalerManager{projectName: projectName, cluster: cluster, directory: directory}
}

// SetUpClusterAutoscaler deploys all resources necessary to set up a Cluster Autoscaler.
func (a *AutoscalerManager) SetUpClusterAutoscaler() error {
	// Create files from templates.
	if err := a.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{Directory: a.directory, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := utils.GetClusterID(a.cluster.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	if err := kc.KubectlApply(clusterAutoscalerDeployment, "-n", envs.Namespace); err != nil {
		return fmt.Errorf("error while applying cluster autoscaler for cluster %s : %w", a.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(a.directory)
}

// DestroyClusterAutoscaler removes all resources regarding Cluster Autoscaler.
func (a *AutoscalerManager) DestroyClusterAutoscaler() error {
	// Create files from templates.
	if err := a.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{Directory: a.directory, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := utils.GetClusterID(a.cluster.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	if err := kc.KubectlDeleteManifest(clusterAutoscalerDeployment, "-n", envs.Namespace); err != nil {
		return fmt.Errorf("error while deleting cluster autoscaler for cluster %s : %w", a.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(a.directory)
}

// generateFiles generates all manifests required for deploying Cluster Autoscaler.
func (a *AutoscalerManager) generateFiles() error {
	tpl := templateUtils.Templates{Directory: a.directory}
	var ca *template.Template
	var err error

	// Load templates
	if ca, err = templateUtils.LoadTemplate(templates.ClusterAutoscalerTemplate); err != nil {
		return fmt.Errorf("error loading cluster autoscaler template : %w", err)
	}
	// Prepare data
	clusterId := utils.GetClusterID(a.cluster.ClusterInfo)
	version, err := getK8sVersion(a.cluster.Kubernetes)
	operatorHostname := utils.GetEnvDefault("OPERATOR_HOSTNAME", defaultOperatorHostname)
	operatorPort := utils.GetEnvDefault("OPERATOR_PORT", defaultOperatorPort)
	if err != nil {
		return err
	}

	caData := &autoscalerDeploymentData{
		ClusterName:       a.cluster.ClusterInfo.Name,
		ProjectName:       a.projectName,
		ClusterID:         clusterId,
		AdapterPort:       defaultAdapterPort,
		KubernetesVersion: version,
		OperatorHostname:  operatorHostname,
		OperatorPort:      operatorPort,
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
