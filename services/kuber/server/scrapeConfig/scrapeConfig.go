package scrapeconfig

import (
	"encoding/base64"
	"fmt"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
)

type ScrapeConfig struct {
	Cluster    *pb.K8Scluster
	LBClusters []*pb.LBcluster
	Directory  string
}

type SCData struct {
	LBClusters []*LBcluster
}

type LBcluster struct {
	NodePools *NodePools
}

type NodePools struct {
	Dynamic []*pb.NodePool
	Static  []*pb.NodePool
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
func (sc *ScrapeConfig) GenerateAndApplyScrapeConfig() error {
	// Generate loadbalancers scrape config
	template := templateUtils.Templates{Directory: sc.Directory}

	// Generate prometheus scrape config to file
	tpl, err := templateUtils.LoadTemplate(templates.ScrapeConfigTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", scrapeConfigFileTpl, sc.Cluster.ClusterInfo.Name, err)
	}
	scrapeConfig, err := template.GenerateToString(tpl, sc.getData())
	if err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scrapeConfigFile, sc.Cluster.ClusterInfo.Name, err)
	}

	// Generate manifest for namespace and secret
	tpl, err = templateUtils.LoadTemplate(templates.ScrapeConfigManifestTemplate)
	if err != nil {
		return fmt.Errorf("error while loading %s on %s: %w", scManifestFileTpl, sc.Cluster.ClusterInfo.Name, err)
	}
	if err = template.Generate(tpl, scManifestFile, ScManifestData{Namespace: scrapeConfigNamespace,
		ScrapeConfigB64: base64.StdEncoding.EncodeToString([]byte(scrapeConfig))}); err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scManifestFile, sc.Cluster.ClusterInfo.Name, err)
	}

	// Apply namespace and secret to the cluster
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig, Directory: sc.Directory}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", sc.Cluster.ClusterInfo.Name, sc.Cluster.ClusterInfo.Hash)
		k.Stdout = comm.GetStdOut(prefix)
		k.Stderr = comm.GetStdErr(prefix)
	}
	if err = k.KubectlApply(scManifestFile); err != nil {
		return fmt.Errorf("error while applying %s on %s: %w", scManifestFile, sc.Cluster.ClusterInfo.Name, err)
	}

	return nil
}

// RemoveIfNoLbScrapeConfig will remove the LB scrape-config.yml
func (sc *ScrapeConfig) RemoveLbScrapeConfig() error {
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", sc.Cluster.ClusterInfo.Name, sc.Cluster.ClusterInfo.Hash)
		k.Stdout = comm.GetStdOut(prefix)
		k.Stderr = comm.GetStdErr(prefix)
	}
	if err := k.KubectlDeleteResource("secret", "loadbalancers-scrape-config", "-n", scrapeConfigNamespace); err != nil {
		return fmt.Errorf("error while removing LB scrape-config on %s: %w", sc.Cluster.ClusterInfo.Name, err)
	}
	return nil
}

func (sc *ScrapeConfig) getData() SCData {
	lbs := make([]*LBcluster, 0, len(sc.LBClusters))
	for _, l := range sc.LBClusters {
		lbs = append(lbs, &LBcluster{NodePools: &NodePools{
			Dynamic: utils.GetCommonDynamicNodePools(l.ClusterInfo.NodePools),
			Static:  utils.GetCommonStaticNodePools(l.ClusterInfo.NodePools),
		}})
	}
	return SCData{LBClusters: lbs}
}
