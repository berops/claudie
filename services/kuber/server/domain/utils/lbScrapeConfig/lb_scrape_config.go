package lbScrapeConfig

import (
	"encoding/base64"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
	kuberGoYAMLTemplates "github.com/berops/claudie/services/kuber/templates"
)

const (
	scrapeConfigTemplateFileName = "scrape-config.goyaml"
	scrapeConfigFileName         = "scrape-config.yaml"

	manifestTemplateFileName = "scrape-config-manifest.goyaml"
	manifestFileName         = "scrape-config-manifest.yaml"

	namespace = "monitoring"
)

type ScrapeConfigTemplateParameters struct {
	LBClusters []*pb.LBcluster
}

type ManifestTemplateParameters struct {
	Namespace       string
	ScrapeConfigB64 string
}

// PrometheusScrapeConfigManagerForLBClusters is responsible for managing the Prometheus scrape
// configuration related to scraping data from LB clusters.
type PrometheusScrapeConfigManagerForLBClusters struct {
	OutputDirectory string

	K8sCluster         *pb.K8Scluster
	AttachedLBClusters []*pb.LBcluster
}

// GenerateAndApplyScrapeConfig will generate a template Kubernetes secret with Prometheus
// configuration, for scraping data from node-exporter endpoints of the LB clusters.
// It will create a Kubernetes namespace called "monitoring" and the Kubernetes secret will be
// applied in that namespace.
// If there is no LB cluster, the Prometheus config will also have no endpoints for scraping.
func (p *PrometheusScrapeConfigManagerForLBClusters) GenerateAndApplyScrapeConfig() error {
	templates := templateUtils.Templates{Directory: p.OutputDirectory}

	template, err := templateUtils.LoadTemplate(kuberGoYAMLTemplates.ScrapeConfigTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", scrapeConfigTemplateFileName, p.K8sCluster.ClusterInfo.Name, err)
	}
	// Generate the Prometheus scrape config
	scrapeConfig, err := templates.GenerateToString(template, ScrapeConfigTemplateParameters{LBClusters: p.AttachedLBClusters})
	if err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scrapeConfigFileName, p.K8sCluster.ClusterInfo.Name, err)
	}

	// Generate manifest file for the Kubernetes namespace and secret.
	template, err = templateUtils.LoadTemplate(kuberGoYAMLTemplates.ScrapeConfigManifestTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", manifestTemplateFileName, p.K8sCluster.ClusterInfo.Name, err)
	}
	if err = templates.Generate(template, manifestFileName,
		ManifestTemplateParameters{
			Namespace:       namespace,
			ScrapeConfigB64: base64.StdEncoding.EncodeToString([]byte(scrapeConfig)),
		},
	); err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", manifestFileName, p.K8sCluster.ClusterInfo.Name, err)
	}

	// Apply the manifest file to the Kubernetes cluster.
	k := kubectl.Kubectl{Kubeconfig: p.K8sCluster.Kubeconfig, Directory: p.OutputDirectory}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", p.K8sCluster.ClusterInfo.Name, p.K8sCluster.ClusterInfo.Hash)
		k.Stdout = comm.GetStdOut(prefix)
		k.Stderr = comm.GetStdErr(prefix)
	}
	if err = k.KubectlApply(manifestFileName); err != nil {
		return fmt.Errorf("error while applying %s on %s: %w", manifestFileName, p.K8sCluster.ClusterInfo.Name, err)
	}

	return nil
}

// RemoveScrapeConfig will delete the Kubernetes secret that was generated for this K8s cluster
// in GenerateAndApplyScrapeConfig.
func (p *PrometheusScrapeConfigManagerForLBClusters) RemoveScrapeConfig() error {
	k := kubectl.Kubectl{Kubeconfig: p.K8sCluster.Kubeconfig, MaxKubectlRetries: 3}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", p.K8sCluster.ClusterInfo.Name, p.K8sCluster.ClusterInfo.Hash)

		k.Stdout = comm.GetStdOut(prefix)
		k.Stderr = comm.GetStdErr(prefix)
	}

	if err := k.KubectlDeleteResource("secret", "loadbalancers-scrape-config", "-n", namespace); err != nil {
		return fmt.Errorf("error while removing LB scrape-config on %s: %w", p.K8sCluster.ClusterInfo.Name, err)
	}

	return nil
}
