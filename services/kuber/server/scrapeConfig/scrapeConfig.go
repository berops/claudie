package scrapeconfig

import (
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

type NSData struct {
	Namespace string
}

const (
	namespaceFileTpl      = "namespace.goyaml"
	namespaceFile         = "namespace.yaml"
	scrapeConfigNamespace = "monitoring"
	scrapeConfigName      = "additional-scrape-config"
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
		return fmt.Errorf("error while loading %s on %s: %w", scrapeConfigFileTpl, sc.Cluster.GetClusterInfo(), err)
	}
	err = template.Generate(tpl, scrapeConfigFile, SCData{LBClusters: sc.LBClusters})
	if err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scrapeConfigFile, sc.Cluster.GetClusterInfo(), err)
	}

	// Generate namespace
	tpl, err = templateLoader.LoadTemplate(namespaceFileTpl)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", namespaceFileTpl, sc.Cluster.GetClusterInfo(), err)
	}
	err = template.Generate(tpl, namespaceFile, NSData{Namespace: scrapeConfigNamespace})
	if err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", namespaceFile, sc.Cluster.GetClusterInfo(), err)
	}

	// Apply namespace and scrape-config to the cluster
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig, Directory: sc.Directory}
	if err = k.KubectlApply(namespaceFile, ""); err != nil {
		return fmt.Errorf("error while applying %s on %s: %w", namespaceFile, sc.Cluster.GetClusterInfo(), err)
	}
	if err = k.KubectlCreateOrPatchSecretFromFile(scrapeConfigName, scrapeConfigFile, scrapeConfigNamespace); err != nil {
		return fmt.Errorf("error while applying %s on %s: %w", scrapeConfigFile, sc.Cluster.GetClusterInfo(), err)
	}

	return nil
}
