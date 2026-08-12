package scrapeconfig

import (
	"encoding/base64"
	"fmt"
	"os"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/tmplutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/templates"
)

type ScrapeConfig struct {
	Cluster    *spec.K8Scluster
	LBClusters []*spec.LBcluster
	Directory  string
}

type SCData struct {
	LBClusters []*LBcluster
}

type LBcluster struct {
	NodePools *NodePools
	Roles     []*spec.Role
}

type NodePools struct {
	Dynamic []*spec.NodePool
	Static  []*spec.NodePool
}

type ScManifestData struct {
	ScrapeConfigB64 string
	Namespace       string
}

const (
	scrapeConfigNamespace = "monitoring"
	scManifestFile        = "scrape-config-manifest.yaml"
	scrapeConfigFile      = "scrape-config.yaml"
)

// GenerateAndApplyScrapeConfig will template a secret with Prometheus configuration,
// for scraping Loadbalancers node-exporter endpoints
// it will create the secret in a applied namespace
// If there is no loadbalancers it will apply the config with no target endpoints
func (sc *ScrapeConfig) GenerateAndApplyScrapeConfig() error {
	if err := os.RemoveAll(sc.Directory); err != nil {
		return fmt.Errorf("failed to remove %q: %w", sc.Directory, err)
	}

	// Generate loadbalancers scrape config
	template := tmplutils.Templates{Directory: sc.Directory}

	// Generate prometheus scrape config to file
	tpl, err := tmplutils.LoadTemplate(templates.ScrapeConfigTemplate)
	if err != nil {
		return fmt.Errorf("error while loading scrape config file for %s: %w", sc.Cluster.ClusterInfo.Name, err)
	}

	scrapeConfig, err := template.GenerateToString(tpl, sc.getData())
	if err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scrapeConfigFile, sc.Cluster.ClusterInfo.Name, err)
	}

	// Generate manifest for namespace and secret
	tpl, err = tmplutils.LoadTemplate(templates.ScrapeConfigManifestTemplate)
	if err != nil {
		return fmt.Errorf("error while loading scrape config template for %s: %w", sc.Cluster.ClusterInfo.Name, err)
	}

	data := ScManifestData{
		Namespace:       scrapeConfigNamespace,
		ScrapeConfigB64: base64.StdEncoding.EncodeToString([]byte(scrapeConfig)),
	}

	defer func() {
		// ignore the error here, as there is no way to handle it
		// nor is there also any logger passed into the function.
		//
		// If the removal of the directory fails due to any reason
		// it shouldn't matter as if there is another call with
		// the same directory it will be caught by the initial
		// os.RemoveAll at the beginning of the function.
		_ = os.RemoveAll(sc.Directory)
	}()

	if err = template.Generate(tpl, scManifestFile, data); err != nil {
		return fmt.Errorf("error while generating %s on %s: %w", scManifestFile, sc.Cluster.ClusterInfo.Name, err)
	}

	// Apply namespace and secret to the cluster
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig, Directory: sc.Directory}
	k.Stdout = comm.GetStdOut(sc.Cluster.ClusterInfo.Id())
	k.Stderr = comm.GetStdErr(sc.Cluster.ClusterInfo.Id())

	if err = k.KubectlApply(scManifestFile); err != nil {
		return fmt.Errorf("error while applying %s on %s: %w", scManifestFile, sc.Cluster.ClusterInfo.Name, err)
	}

	return nil
}

// RemoveIfNoLBScrapeConfig will remove the LB scrape-config.yml
func (sc *ScrapeConfig) RemoveLBScrapeConfig() error {
	k := kubectl.Kubectl{Kubeconfig: sc.Cluster.Kubeconfig, MaxKubectlRetries: 3}
	k.Stdout = comm.GetStdOut(sc.Cluster.ClusterInfo.Id())
	k.Stderr = comm.GetStdErr(sc.Cluster.ClusterInfo.Id())

	if err := k.KubectlDeleteResource("secret", "loadbalancers-scrape-config", "-n", scrapeConfigNamespace); err != nil {
		return fmt.Errorf("error while removing LB scrape-config on %s: %w", sc.Cluster.ClusterInfo.Name, err)
	}
	return nil
}

func (sc *ScrapeConfig) getData() SCData {
	lbs := make([]*LBcluster, 0, len(sc.LBClusters))
	for _, l := range sc.LBClusters {
		lbs = append(lbs, &LBcluster{
			NodePools: &NodePools{
				Dynamic: nodepools.Dynamic(l.ClusterInfo.NodePools),
				Static:  nodepools.Static(l.ClusterInfo.NodePools),
			},
			Roles: l.Roles,
		})
	}
	return SCData{LBClusters: lbs}
}
