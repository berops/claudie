package autoscaler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"text/template"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/templates"
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
	// CA registry address to query for tags
	clusterAutoscalerRegistry = "https://registry.k8s.io/v2/autoscaling/cluster-autoscaler/tags/list"
)

// ClusterAutoscalerManager either creates or destroys Cluster Autoscaler resources for given k8s cluster.
type AutoscalerManager struct {
	// Project name where k8s cluster is defined.
	projectName string
	// K8s cluster.
	cluster *spec.K8Scluster
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

// TagResponse will carry the information about cluster-autoscaler tags
type TagsResponse struct {
	Tags []string `json:"tags"`
}

// NewAutoscalerManager returns configured AutoscalerManager which can set up or remove Cluster Autoscaler.
func NewAutoscalerManager(projectName string, cluster *spec.K8Scluster, directory string) *AutoscalerManager {
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
	kc.Stdout = comm.GetStdOut(a.cluster.ClusterInfo.Id())
	kc.Stderr = comm.GetStdErr(a.cluster.ClusterInfo.Id())

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
	kc.Stdout = comm.GetStdOut(a.cluster.ClusterInfo.Id())
	kc.Stderr = comm.GetStdErr(a.cluster.ClusterInfo.Id())

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
	clusterId := a.cluster.ClusterInfo.Id()
	version, err := getK8sVersion(a.cluster.Kubernetes)
	operatorHostname := envs.GetOrDefault("OPERATOR_HOSTNAME", defaultOperatorHostname)
	operatorPort := envs.GetOrDefault("OPERATOR_PORT", defaultOperatorPort)
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
		latestVersion, err := getLatestMinorVersion(version)
		if err != nil {
			log.Warn().Msg("Could not retrieve latest cluster-autoscaler version, fallback to defined Kubernetes version")
			return fmt.Sprintf("v%s.%s.%s", match[1], match[2], match[3]), nil
		}
		return latestVersion, nil
	}
	return "", fmt.Errorf("failed to parse %s into autoscaler image tag [vX.Y.Z]", version)
}

// getLatestMinorVersion returns latest patch tag for given kubernetes version
// Example: for v1.25.1 returns v1.25.3
func getLatestMinorVersion(k8sVersion string) (string, error) {
	var minor, patch int

	tagList, err := getClusterAutoscaleVersions()
	if err != nil {
		return "", err
	}

	// Extract values from k8sVersion
	_, err = fmt.Sscanf(k8sVersion, "1.%d.%d", &minor, &patch)
	if err != nil {
		return "", err
	}

	// Find latest patch version
	var latestPatch int
	for _, semver := range tagList {
		var listMinor, listPatch int
		_, err := fmt.Sscanf(semver, "v1.%d.%d", &listMinor, &listPatch)
		if err != nil {
			return "", err
		}
		if minor == listMinor {
			if listPatch > latestPatch {
				latestPatch = listPatch
			}
		}
	}

	latestSemver := fmt.Sprintf("v1.%d.%d", minor, latestPatch)

	return latestSemver, nil
}

// getClusterAutoscaleVersions query the CA registry to retrieve all available tags,
// extracts proper semver from it, and returns a slice of available tags
func getClusterAutoscaleVersions() ([]string, error) {
	var TagsResponse TagsResponse

	// Query CA registry
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, clusterAutoscalerRegistry, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Unmarshal tags from the response
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &TagsResponse)
	if err != nil {
		return nil, err
	}

	// Extract semver from the slice
	var semverList []string
	semverRegex := regexp.MustCompile(`v\d+\.\d+\.\d+(-[\w\d]+(\.[\w\d]+)*)?`)

	// Iterate through the input slice and filter out non-semver strings
	for _, str := range TagsResponse.Tags {
		semverMatches := semverRegex.FindStringSubmatch(str)
		if len(semverMatches) > 0 {
			semverList = append(semverList, semverMatches[0])
		}
	}

	return semverList, nil
}
