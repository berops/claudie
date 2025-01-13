package cluster_builder

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/generics"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/terraform"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

const (
	TemplatesRootDir = "services/terraformer/templates"
	Output           = "services/terraformer/server/clusters"
)

type K8sInfo struct{ ExportPort6443 bool }
type LBInfo struct{ Roles []*spec.Role }

// ClusterBuilder wraps data needed for building a cluster.
type ClusterBuilder struct {
	// DesiredClusterInfo contains the information about the
	// desired state of the cluster.
	DesiredClusterInfo *spec.ClusterInfo
	// CurrentClusterInfo contains the information about the
	// current state of the cluster.
	CurrentClusterInfo *spec.ClusterInfo
	// ProjectName is the name of the manifest.
	ProjectName string
	// ClusterType is the type of the cluster being build
	// LoadBalancer or K8s.
	ClusterType spec.ClusterType
	// K8sInfo contains additional data for when building kubernetes clusters.
	K8sInfo K8sInfo
	// LBInfo contains additional data for when building loadbalancer clusters.
	LBInfo LBInfo
	// SpawnProcessLimit limits the number of spawned terraform processes.
	SpawnProcessLimit *semaphore.Weighted
}

// CreateNodepools creates node pools for the cluster.
func (c ClusterBuilder) CreateNodepools() error {
	clusterID := c.DesiredClusterInfo.Id()
	clusterDir := filepath.Join(Output, clusterID)

	defer func() {
		// Clean after terraform
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting files in %s : %v", clusterDir, err)
		}
	}()

	if err := c.generateFiles(clusterID, clusterDir); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	terraform := terraform.Terraform{
		Directory:         clusterDir,
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	terraform.Stdout = comm.GetStdOut(clusterID)
	terraform.Stderr = comm.GetStdErr(clusterID)

	if err := terraform.Init(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	var currentState []string
	if c.CurrentClusterInfo != nil {
		var err error
		if currentState, err = terraform.StateList(); err != nil {
			return fmt.Errorf("error while running terraform state list in %s : %w", clusterID, err)
		}
	}

	if err := terraform.Apply(); err != nil {
		updatedState, errList := terraform.StateList()
		if errList != nil {
			return errors.Join(err, fmt.Errorf("%w: error while running terraform state list in %s : %w", err, clusterID, errList))
		}

		var toDelete []string
		for _, resource := range updatedState {
			if !slices.Contains(currentState, resource) {
				toDelete = append(toDelete, resource)
			}
		}

		if errDestroy := terraform.DestroyTarget(toDelete); errDestroy != nil {
			return fmt.Errorf("%w: failed to destroy partially create state: %w", err, errDestroy)
		}

		return err
	}

	// TODO: remove DEBUG PRINT
	fmt.Printf("before----------------------------------->")
	for _, np := range c.DesiredClusterInfo.NodePools {
		for _, n := range np.Nodes {
			fmt.Printf("%s private (%q) public (%q)\n", n.Name, n.Private, n.Public)
		}
	}
	fmt.Println()

	for _, nodepool := range nodepools.Dynamic(c.DesiredClusterInfo.NodePools) {
		d := nodepool.GetDynamicNodePool()
		f := hash.Digest128(filepath.Join(d.Provider.SpecName, d.Provider.Templates.MustExtractTargetPath()))
		k := fmt.Sprintf("%s_%s_%s", nodepool.Name, d.Provider.SpecName, hex.EncodeToString(f))

		output, err := terraform.Output(k)
		if err != nil {
			return fmt.Errorf("error while getting output from terraform for %s : %w", nodepool.Name, err)
		}
		out, err := readIPs(output)
		if err != nil {
			return fmt.Errorf("error while reading the terraform output for %s : %w", nodepool.Name, err)
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
				return fmt.Errorf("node %s from nodepool %s was missing from the terraform output, possibly the VM was not properly created", n.Name, nodepool.Name)
			}
		}
	}

	// TODO: remove DEBUG PRINT
	fmt.Printf("after------------------------------------>")
	for _, np := range c.DesiredClusterInfo.NodePools {
		for _, n := range np.Nodes {
			fmt.Printf("%s private (%q) public (%q)\n", n.Name, n.Private, n.Public)
		}
	}
	fmt.Println()

	return nil
}

// DestroyNodepools destroys nodepools for the cluster.
func (c ClusterBuilder) DestroyNodepools() error {
	clusterID := c.CurrentClusterInfo.Id()
	clusterDir := filepath.Join(Output, clusterID)

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting files in %s : %v", clusterDir, err)
		}
	}()

	if err := c.generateFiles(clusterID, clusterDir); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	terraform := terraform.Terraform{
		Directory:         clusterDir,
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	terraform.Stdout = comm.GetStdOut(clusterID)
	terraform.Stderr = comm.GetStdErr(clusterID)

	if err := terraform.Init(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	if err := terraform.Destroy(); err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %w", clusterID, err)
	}

	return nil
}

// generateFiles creates all the necessary terraform files used to create/destroy node pools.
func (c *ClusterBuilder) generateFiles(clusterID, clusterDir string) error {
	backend := templates.Backend{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := backend.CreateTFFile(); err != nil {
		return err
	}

	// generate Providers terraform configuration
	usedProviders := templates.UsedProviders{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := usedProviders.CreateUsedProvider(c.CurrentClusterInfo, c.DesiredClusterInfo); err != nil {
		return err
	}

	var clusterInfo *spec.ClusterInfo
	if c.DesiredClusterInfo != nil {
		clusterInfo = c.DesiredClusterInfo
	} else if c.CurrentClusterInfo != nil {
		clusterInfo = c.CurrentClusterInfo
	}

	clusterData := templates.ClusterData{
		ClusterName: clusterInfo.Name,
		ClusterHash: clusterInfo.Hash,
		ClusterType: c.ClusterType.String(),
	}

	if err := c.generateProviderTemplates(c.CurrentClusterInfo, c.DesiredClusterInfo, clusterID, clusterDir, clusterData); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	for info, pools := range nodepools.ByProviderDynamic(clusterInfo.NodePools) {
		templatesDownloadDir := filepath.Join(TemplatesRootDir, clusterID, info.SpecName)

		for path, pools := range nodepools.ByTemplates(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()

			if err := templates.DownloadProvider(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("cluster %q failed to download template repository", clusterID)
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
				ID:                clusterID,
				TargetDirectory:   clusterDir,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      path,
				Fingerprint:       hex.EncodeToString(hash.Digest128(filepath.Join(info.SpecName, path))),
			}

			if err := g.GenerateNetworking(&templates.Networking{
				ClusterData: clusterData,
				Provider:    p,
				Regions:     nodepools.ExtractRegions(nodepools.ExtractDynamic(pools)),
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

// readIPs reads json output format from terraform and unmarshal it into map[string]map[string]string readable by Go.
func readIPs(data string) (templates.NodepoolIPs, error) {
	var result templates.NodepoolIPs
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
	return result, err
}

// generateProviderTemplates generates only the `provider.tpl` templates so terraform can destroy the infra if needed.
func (c *ClusterBuilder) generateProviderTemplates(current, desired *spec.ClusterInfo, clusterID, directory string, clusterData templates.ClusterData) error {
	nps := slices.AppendSeq(
		slices.Collect(slices.Values(current.GetNodePools())),
		slices.Values(desired.GetNodePools()),
	)

	for info, pools := range nodepools.ByProviderDynamic(nps) {
		if err := fileutils.CreateKey(info.Creds, directory, info.SpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", info.SpecName, directory, err)
		}

		templatesDownloadDir := filepath.Join(TemplatesRootDir, clusterID, info.SpecName)

		for path, pools := range nodepools.ByTemplates(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()
			if err := templates.DownloadProvider(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("cluster %q failed to download template repository", clusterID)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			g := templates.Generator{
				ID:                clusterID,
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
