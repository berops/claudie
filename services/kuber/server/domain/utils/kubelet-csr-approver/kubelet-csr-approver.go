package kubeletcsrapprover

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/templates"
)

const (
	kubeletCSRApproverDeployment = "kubelet-csr-approver.yaml"
)

// KubeletCSRApprover either deploys or deletes kubelet-csr-approver resources for given k8s cluster.
type KubeletCSRApprover struct {
	// Project name where k8s cluster is defined.
	projectName string
	// K8s cluster.
	cluster *spec.K8Scluster
	// Output directory.
	directory string
}

type kubeletCSRApproverDeploymentData struct {
	ClusterName        string
	ProjectName        string
	ClusterID          string
	ProviderRegex      string
	ProviderIPPrefixes string
}

// NewKubeletCSRApprover returns configured KubeletCSRApprover which can set up deploy or delete kubelet-csr-approver.
func NewKubeletCSRApprover(projectName string, cluster *spec.K8Scluster, directory string) *KubeletCSRApprover {
	return &KubeletCSRApprover{projectName: projectName, cluster: cluster, directory: directory}
}

func (kca *KubeletCSRApprover) DeployKubeletCSRApprover() error {
	// Create files from templates.
	if err := kca.generateFiles(); err != nil {
		return err
	}
	// Apply generated files.
	kc := kubectl.Kubectl{
		Kubeconfig:        kca.cluster.Kubeconfig,
		Directory:         kca.directory,
		MaxKubectlRetries: 3,
	}
	kc.Stdout = comm.GetStdOut(kca.cluster.ClusterInfo.Id())
	kc.Stderr = comm.GetStdErr(kca.cluster.ClusterInfo.Id())

	// deploys to namespace defined in the template (should be kube-system by default)
	if err := kc.KubectlApply(kubeletCSRApproverDeployment, ""); err != nil {
		return fmt.Errorf("error while applying kubelet-csr-approver for cluster %s : %w", kca.cluster.ClusterInfo.Name, err)
	}
	return os.RemoveAll(kca.directory)
}

// generateFiles generates all manifests required for deploying Cluster Autoscaler.
func (k *KubeletCSRApprover) generateFiles() error {
	tpl := templateUtils.Templates{Directory: k.directory}
	var kcrTemplate *template.Template
	var err error

	// Load templates
	// The configuration files for templates were taken from https://github.com/postfinance/kubelet-csr-approver/tree/v1.2.12/deploy/k8s
	if kcrTemplate, err = templateUtils.LoadTemplate(templates.KubeletCSRApproverTemplate); err != nil {
		return fmt.Errorf("error loading kubelet-csr-approver template : %w", err)
	}

	var parts []string
	nodepools := k.cluster.ClusterInfo.GetNodePools()
	for _, nodepool := range nodepools {
		parts = append(parts, regexp.QuoteMeta(nodepool.Name))
	}

	regexPattern := fmt.Sprintf("^(%s)-.+$", strings.Join(parts, "|"))

	kubeletCSRApproverData := &kubeletCSRApproverDeploymentData{
		ClusterName:        k.cluster.ClusterInfo.Name,
		ProjectName:        k.projectName,
		ClusterID:          k.cluster.ClusterInfo.Id(),
		ProviderRegex:      regexPattern,
		ProviderIPPrefixes: k.cluster.Network,
	}

	if err := tpl.Generate(kcrTemplate, kubeletCSRApproverDeployment, kubeletCSRApproverData); err != nil {
		return fmt.Errorf("error generating kubelet-csr-approver deployment : %w", err)
	}

	return nil
}
