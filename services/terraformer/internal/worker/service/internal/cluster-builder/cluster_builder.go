package cluster_builder

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/generics"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/templates"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/tofu"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

// Supported Cluster Type by the Cluster Builder.
type ClusterType string

const (
	Kubernetes   ClusterType = "K8s"
	LoadBalancer ClusterType = "LB"
)

const (
	TemplatesRootDir = "services/terraformer/templates"
	Output           = "services/terraformer/clusters"
	CacheDir         = "services/terraformer/cache"
)

type K8sInfo struct{ ExportPort6443 bool }
type LBInfo struct{ Roles []*spec.Role }

type ClusterBuilder struct {
	ClusterName string
	ClusterHash string
	ClusterId   string

	// NodePools that represent the actuall state of the
	// infrastructure, these are the nodepools that should
	// be build when calling Tofu.Apply or destroyed
	// when calling Tofu.Destroy
	NodePools []*spec.NodePool

	// GhostNodepools are nodepools that were removed from
	// the [ClusterBuilder.NodePools] state, but not yet from
	// the state file, terraformer still needs to know about them
	// to correctly clean up the terraform state. This field should
	// only be used whenever the need to generate the provider for
	// the 'Removed' nodepools should be generated so that the next
	// Tofu.Apply will result in the deletion of the resources of
	// that nodepool.
	GhostNodePools []*spec.NodePool

	// ProjectName is the name of the manifest.
	ProjectName string

	// ClusterType is the type of the cluster being build
	// LoadBalancer or K8s.
	ClusterType ClusterType

	// K8sInfo contains additional data for when building kubernetes clusters.
	K8sInfo K8sInfo

	// LBInfo contains additional data for when building loadbalancer clusters.
	LBInfo LBInfo

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

// CreateNodepools creates node pools for the cluster.
func (c ClusterBuilder) ReconcileNodePools() error {
	clusterDir := filepath.Join(Output, c.ClusterId)

	defer func() {
		// Clean after tofu
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting files in %s : %v", clusterDir, err)
		}
	}()

	if err := c.generateFiles(clusterDir); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	tofu := tofu.Terraform{
		Directory:         clusterDir,
		SpawnProcessLimit: c.SpawnProcessLimit,
		CacheDir:          CacheDir,
	}

	tofu.Stdout = comm.GetStdOut(c.ClusterId)
	tofu.Stderr = comm.GetStdErr(c.ClusterId)

	if err := tofu.ProvidersLock(); err != nil {
		log.Warn().Msgf("Error while locking providers from local FS mirror\n" +
			"Continue to retrieve providers and generate hash from remote registry.")
	}

	if err := tofu.Init(); err != nil {
		return fmt.Errorf("error while running tofu init in %s : %w", c.ClusterId, err)
	}

	if err := tofu.Apply(); err != nil {
		return err
	}

	for _, nodepool := range nodepools.Dynamic(c.NodePools) {
		d := nodepool.GetDynamicNodePool()
		f := hash.Digest128(filepath.Join(d.Provider.SpecName, d.Provider.Templates.MustExtractTargetPath()))
		k := fmt.Sprintf("%s_%s_%s", nodepool.Name, d.Provider.SpecName, hex.EncodeToString(f))

		output, err := tofu.Output(k)
		if err != nil {
			return fmt.Errorf("error while getting output from tofu for %s : %w", nodepool.Name, err)
		}
		out, err := readIPs(output)
		if err != nil {
			return fmt.Errorf("error while reading the tofu output for %s : %w", nodepool.Name, err)
		}
		for _, n := range nodepool.Nodes {
			var found bool
			for target, ip := range generics.IterateMapInOrder(out.IPs) {
				if target != n.Name {
					continue
				}
				found = true
				n.Public = fmt.Sprint(ip)
				break
			}
			if !found {
				return fmt.Errorf("node %s from nodepool %s was missing from the tofu output, possibly the VM was not properly created", n.Name, nodepool.Name)
			}
		}
	}

	return nil
}

// DestroyNodepools destroys nodepools for the cluster.
func (c ClusterBuilder) DestroyNodepools() error {
	var (
		clusterDir = filepath.Join(Output, c.ClusterId)
		tofu       = tofu.Terraform{
			Directory:         clusterDir,
			SpawnProcessLimit: c.SpawnProcessLimit,
			CacheDir:          CacheDir,
		}
	)

	tofu.Stdout = comm.GetStdOut(c.ClusterId)
	tofu.Stderr = comm.GetStdErr(c.ClusterId)

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting files in %s : %v", clusterDir, err)
		}
	}()

	if err := c.generateFiles(clusterDir); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	if err := tofu.ProvidersLock(); err != nil {
		log.Warn().Msgf("Error while locking providers from local FS mirror\n" +
			"Continue to retrieve providers and generate hash from remote registry.")
	}

	if err := tofu.Init(); err != nil {
		return fmt.Errorf("error while running tofu init in %s : %w", c.ClusterId, err)
	}

	if err := tofu.Destroy(); err != nil {
		return fmt.Errorf("error while running tofu apply in %s : %w", c.ClusterId, err)
	}

	return nil
}

// generateFiles creates all the necessary tofu files used to create/destroy node pools.
func (c *ClusterBuilder) generateFiles(clusterDir string) error {
	backend := templates.Backend{
		ProjectName: c.ProjectName,
		ClusterName: c.ClusterId,
		Directory:   clusterDir,
	}

	if err := backend.CreateTFFile(); err != nil {
		return err
	}

	// generate Providers tofu configuration
	usedProviders := templates.UsedProviders{
		ProjectName: c.ProjectName,
		ClusterName: c.ClusterId,
		Directory:   clusterDir,
	}

	// Create providers for all of the nodepools.
	err := usedProviders.CreateUsedProvider(append(c.NodePools, c.GhostNodePools...))
	if err != nil {
		return err
	}

	clusterData := templates.ClusterData{
		ClusterName: c.ClusterName,
		ClusterHash: c.ClusterHash,
		ClusterType: string(c.ClusterType),
	}

	if err := c.generateProviderTemplates(clusterDir, clusterData); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	for info, pools := range nodepools.ByProviderDynamic(c.NodePools) {
		templatesDownloadDir := filepath.Join(TemplatesRootDir, c.ClusterId, info.SpecName)

		for path, pools := range nodepools.ByTemplatesPath(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()

			if err := templates.DownloadProvider(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("cluster %q failed to download template repository", c.ClusterId)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			nps := make([]templates.NodePoolInfo, 0, len(pools))

			for _, np := range pools {
				if dnp := np.GetDynamicNodePool(); dnp != nil {
					nps = append(nps, templates.NodePoolInfo{
						Name:      np.Name,
						Nodes:     np.Nodes,
						Details:   np.GetDynamicNodePool(),
						IsControl: np.IsControl,
					})

					if err := fileutils.CreateKey(dnp.GetPublicKey(), clusterDir, np.GetName()); err != nil {
						return fmt.Errorf("error public key file for %s : %w", clusterDir, err)
					}
				}
			}

			// based on the cluster type fill out the nodepools data to be used
			nodepoolData := templates.Nodepools{
				ClusterData: clusterData,
				NodePools:   nps,
			}

			g := templates.Generator{
				ID:                c.ClusterId,
				TargetDirectory:   clusterDir,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      path,
				Fingerprint:       hex.EncodeToString(hash.Digest128(filepath.Join(info.SpecName, path))),
			}

			if err := g.GenerateNetworking(&templates.Networking{
				ClusterData:   clusterData,
				Provider:      p,
				Regions:       nodepools.ExtractRegions(nodepools.ExtractDynamic(pools)),
				RegionNetwork: nodepools.ExtractRegionNetwork(nodepools.ExtractDynamic(pools)),
				K8sData: templates.K8sData{
					HasAPIServer: c.K8sInfo.ExportPort6443,
				},
				LBData: templates.LBData{
					Roles: c.LBInfo.Roles,
				},
			}); err != nil {
				return fmt.Errorf("failed to generate networking_common template files: %w", err)
			}

			if err := g.GenerateNodes(&nodepoolData); err != nil {
				return fmt.Errorf("failed to generate nodepool specific templates files: %w", err)
			}
		}
	}

	return nil
}

// readIPs reads json output format from tofu and unmarshal it into map[string]map[string]string readable by Go.
func readIPs(data string) (templates.NodepoolIPs, error) {
	var result templates.NodepoolIPs
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
	return result, err
}

// generateProviderTemplates generates only the `provider.tpl` templates so tofu can destroy the infra if needed.
func (c *ClusterBuilder) generateProviderTemplates(directory string, clusterData templates.ClusterData) error {
	// Need to append also the nodepools that are no longer present in the infrastructure
	// so that their statefile records will get cleaned up.
	nps := append(c.NodePools, c.GhostNodePools...)

	for info, pools := range nodepools.ByProviderDynamic(nps) {
		if err := fileutils.CreateKey(info.Creds, directory, info.SpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", info.SpecName, directory, err)
		}

		templatesDownloadDir := filepath.Join(TemplatesRootDir, c.ClusterId, info.SpecName)

		for path, pools := range nodepools.ByTemplatesPath(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()
			if err := templates.DownloadProvider(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("cluster %q failed to download template repository", c.ClusterId)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			g := templates.Generator{
				ID:                c.ClusterId,
				TargetDirectory:   directory,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      path,
				Fingerprint:       hex.EncodeToString(hash.Digest128(filepath.Join(info.SpecName, path))),
			}

			err := g.GenerateProvider(&templates.Provider{
				ClusterData: clusterData,
				Provider:    pools[0].GetDynamicNodePool().GetProvider(),
				Regions:     nodepools.ExtractRegions(nodepools.ExtractDynamic(pools)),
			})

			if err != nil {
				return fmt.Errorf("failed to generate provider templates: %w", err)
			}
		}
	}
	return nil
}
