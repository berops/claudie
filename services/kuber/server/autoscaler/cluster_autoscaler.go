package autoscaler

import (
	"fmt"
	"os"
	"text/template"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
)

const (
	clusterAutoscalerTemplate   = "cluster-autoscaler.goyaml"
	autoscalerAdapterTemplate   = "autoscaler-adapter.goyaml"
	clusterAutoscalerDeployment = "ca.yaml"
	autoscalerAdapterDeployment = "aa.yaml"
)

type AutoscalerBuilder struct {
	projectName string
	cluster     *pb.K8Scluster
	directory   string
}

type AutoscalerDeploymentData struct {
	ClusterID string
}

type AutoscalerAdapterDeploymentData struct {
	ClusterID   string
	ClusterName string
	ProjectName string
}

func NewAutoscalerBuilder(projectName string, cluster *pb.K8Scluster, directory string) *AutoscalerBuilder {
	return &AutoscalerBuilder{projectName: projectName, cluster: cluster, directory: directory}
}

func (ab *AutoscalerBuilder) SetUpClusterAutoscaler() error {
	// Create files from templates.
	if err := ab.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{Directory: ab.directory}
	if err := kc.KubectlApply(autoscalerAdapterDeployment, envs.Namespace); err != nil {
		return fmt.Errorf("error while applying autoscaler adapter for cluster %s : %w", ab.cluster.ClusterInfo.Name, err)
	}
	if err := kc.KubectlApply(clusterAutoscalerDeployment, envs.Namespace); err != nil {
		return fmt.Errorf("error while applying cluster autoscaler for cluster %s : %w", ab.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(ab.directory)
}

func (ab *AutoscalerBuilder) DestroyClusterAutoscaler() error {
	// Create files from templates.
	if err := ab.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{Directory: ab.directory}
	if err := kc.KubectlDeleteManifest(autoscalerAdapterDeployment, envs.Namespace); err != nil {
		return fmt.Errorf("error while deleting autoscaler adapter for cluster %s : %w", ab.cluster.ClusterInfo.Name, err)
	}
	if err := kc.KubectlDeleteManifest(clusterAutoscalerDeployment, envs.Namespace); err != nil {
		return fmt.Errorf("error while deleting cluster autoscaler for cluster %s : %w", ab.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(ab.directory)
}

func (ab *AutoscalerBuilder) generateFiles() error {
	tpl := templateUtils.Templates{Directory: ab.directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.KuberTemplates}
	var ca, aa *template.Template
	var err error

	// Load templates
	if ca, err = templateLoader.LoadTemplate(clusterAutoscalerTemplate); err != nil {
		return fmt.Errorf("error loading cluster autoscaler template : %w", err)
	}
	if aa, err = templateLoader.LoadTemplate(autoscalerAdapterTemplate); err != nil {
		return fmt.Errorf("error loading autoscaler adapter template : %w", err)
	}
	// Prepare data
	clusterId := fmt.Sprintf("%s-%s", ab.cluster.ClusterInfo.Name, ab.cluster.ClusterInfo.Hash)
	aaData := &AutoscalerAdapterDeploymentData{ClusterName: ab.cluster.ClusterInfo.Name, ProjectName: ab.projectName, ClusterID: clusterId}
	caData := &AutoscalerDeploymentData{ClusterID: clusterId}

	// Generate files
	if err := tpl.Generate(aa, autoscalerAdapterDeployment, aaData); err != nil {
		return fmt.Errorf("error generating autoscaler adapter deployment : %w", err)
	}
	if err := tpl.Generate(ca, clusterAutoscalerDeployment, caData); err != nil {
		return fmt.Errorf("error generating cluster autoscaler deployment : %w", err)
	}

	return nil
}
