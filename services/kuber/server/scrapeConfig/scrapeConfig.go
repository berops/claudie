package scrapeconfig

import (
	"encoding/base64"
	"fmt"

	"github.com/Berops/claudie/internal/kubectl"
	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/proto/pb"
)

type ScrapeConfig struct {
	Cluster    *pb.K8Scluster
	LBClusters []*pb.LBcluster
	Directory  string
}

type SCData struct {
	LBClusters []*pb.LBcluster
}

type ScManifestData struct {
	ScrapeConfigB64 string
	Namespace       string
}

const (
	scrapeConfigNamespace = "monitoring"
	scManifestFileTpl     = "scrape-config-manifest.goyaml"
	scManifestFile        = "scrape-config-manifest.yaml"
	scrapeConfigFileTpl   = "scrape-config.goyaml"
	scrapeConfigFile      = "scrape-config.yaml"
)

// GenerateAndApplyScrapeConfig will template a secret with Prometheus configuration,
// for scraping Loadbalancers node-exporter endpoints
// it will create the secret in a applied namespace
// If there is no loadbalancers it will apply the config with no target endpoints
func (sc ScrapeConfig) GenerateAndApplyScrapeConfig() error {
	// Generate loadbalancers scrape config
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.KuberTemplates}
	template := templateUtils.Templates{Directory: sc.Directory}

	// Generate prometheus scrape config to file
	tpl, err := templateLoader.LoadTemplate(scrapeConfigFileTpl)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", scrapeConfigFileTpl, sc.Cluster.ClusterInfo.Name, err)
	}
	scrapeConfig, err := template.GenerateToString(tpl, SCData{LBClusters: sc.LBClusters})
	if err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scrapeConfigFile, sc.Cluster.ClusterInfo.Name, err)
	}

	// Generate manifest for namespace and secret
	tpl, err = templateLoader.LoadTemplate(scManifestFileTpl)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", scManifestFileTpl, sc.Cluster.ClusterInfo.Name, err)
	}
	if err = template.Generate(tpl, scManifestFile, ScManifestData{Namespace: scrapeConfigNamespace,
		ScrapeConfigB64: base64.StdEncoding.EncodeToString([]byte(scrapeConfig))}); err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scManifestFile, sc.Cluster.ClusterInfo.Name, err)
	}

	// Apply namespace and secret to the cluster
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig, Directory: sc.Directory}
	if err = k.KubectlApply(scManifestFile); err != nil {
		return fmt.Errorf("error while applying %s on %s: %w", scManifestFile, sc.Cluster.ClusterInfo.Name, err)
	}

	return nil
}

// RemoveIfNoLbScrapeConfig will remove the LB scrape-config.yml
func (sc ScrapeConfig) RemoveLbScrapeConfig() error {
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig}
	if err := k.KubectlDeleteResource("secret", "loadbalancers-scrape-config", "-n", scrapeConfigNamespace); err != nil {
		return fmt.Errorf("error while removing LB scrape-config on %s: %w", sc.Cluster.ClusterInfo.Name, err)
	}
	return nil
}
