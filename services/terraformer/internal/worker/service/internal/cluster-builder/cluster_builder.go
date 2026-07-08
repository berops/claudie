package cluster_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/extemplates"
	"github.com/berops/claudie/internal/extemplates/extofu"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/generics"
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
		output, err := tofu.Output(extofu.NodePoolTerraformKey(nodepool))
		if err != nil {
			return fmt.Errorf("error while getting output from tofu for %s : %w", nodepool.Name, err)
		}
		out, err := readIPs(output)
		if err != nil {
			return fmt.Errorf("error while reading the tofu output for %s : %w", nodepool.Name, err)
		}
		for _, n := range nodepool.Nodes {
			var found bool
			for target, val := range generics.IterateMapInOrder(out.IPs) {
				if target != n.Name {
					continue
				}
				ip, sshPort, wgPort, err := parseNodeOutput(val)
				if err != nil {
					return fmt.Errorf("node %q from nodepool %q: %w", n.Name, nodepool.Name, err)
				}
				if ip == "" {
					return fmt.Errorf("node %q from nodepool %q has no public address assigned", n.Name, nodepool.Name)
				}
				found = true
				n.Public = ip
				if sshPort > 0 {
					n.SshPort = sshPort
				}
				if wgPort > 0 {
					n.WireguardPort = wgPort
				}
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
		if errors.Is(err, extemplates.ErrUnknownCommit) {
			log.
				Warn().
				Msgf("Failed to generate files for nodepool destruction: %v,"+
					" since the commit of one of the templates does no exist, leaking infrastructure", err)
			return nil
		}
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

	clusterData := extofu.ClusterData{
		ClusterName: c.ClusterName,
		ClusterHash: c.ClusterHash,
		ClusterType: string(c.ClusterType),
	}

	if err := c.generateProviderTemplates(clusterDir, clusterData); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	for info, pools := range nodepools.ByProviderDynamic(c.NodePools) {
		templatesDownloadDir := filepath.Join(TemplatesRootDir, c.ClusterId, info.SpecName)

		for _, pools := range extofu.NodePoolsByTemplatesPath(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()

			if err := extofu.Download(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("failed to setup template repository for cluster %q, provider %q", c.ClusterId, p.SpecName)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			nps := make([]extofu.NodePoolInfo, 0, len(pools))

			for _, np := range pools {
				if dnp := np.GetDynamicNodePool(); dnp != nil {
					nps = append(nps, extofu.NodePoolInfo{
						Name:      np.Name,
						Nodes:     np.Nodes,
						Details:   dnp,
						IsControl: np.IsControl,
					})

					if err := fileutils.CreateKey(dnp.GetPublicKey(), clusterDir, np.GetName()); err != nil {
						return fmt.Errorf("error public key file for %s : %w", clusterDir, err)
					}
				}
			}

			dyn := nodepools.ExtractDynamic(pools)
			reg := nodepools.ExtractRegions(dyn)
			var rgn []extofu.RegionNetwork

			for _, v := range nodepools.ExtractRegionNetwork(dyn) {
				rgn = append(rgn, extofu.RegionNetwork(v))
			}

			g := extofu.Generator{
				ID:                c.ClusterId,
				TargetDirectory:   clusterDir,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      extofu.TemplatesPath(p),
				Fingerprint:       extofu.Fingerprint(p),
			}

			n := extofu.Networking{
				ClusterData:   clusterData,
				Provider:      p,
				Regions:       reg,
				RegionNetwork: rgn,
				K8sData: extofu.K8sData{
					HasAPIServer: c.K8sInfo.ExportPort6443,
				},
				LBData: extofu.LBData{
					Roles: c.LBInfo.Roles,
				},
			}

			nodepoolData := extofu.Nodepools{
				ClusterData: clusterData,
				NodePools:   nps,
			}

			if err := g.GenerateNetworking(&n); err != nil {
				return fmt.Errorf("failed to generate networking_common template files: %w", err)
			}

			if err := g.GenerateNodes(&nodepoolData); err != nil {
				return fmt.Errorf("failed to generate nodepool specific templates files: %w", err)
			}
		}
	}

	return nil
}

// parseNodeOutput extracts the IP and optional per-node SSH and WireGuard ports
// from a terraform output value. Templates may output any of:
//   - a string (just the IP)
//   - [IP, sshPort]
//   - [IP, sshPort, wireguardPort]
//
// The ports are used by shared-IP / NAT nodes (e.g. CloudRift) where each VM is
// reached on its own mapped host port. A zero/absent port means "use the default".
func parseNodeOutput(val any) (ip string, sshPort, wgPort int32, err error) {
	switch v := val.(type) {
	case string:
		return v, 0, 0, nil
	case []any:
		if len(v) == 0 {
			return "", 0, 0, fmt.Errorf("empty output array")
		}
		ipStr, ok := v[0].(string)
		if !ok || ipStr == "" {
			return "", 0, 0, fmt.Errorf("invalid IP value type %T", v[0])
		}
		if len(v) >= 2 {
			sshPort = parsePort(v[1])
		}
		if len(v) >= 3 {
			wgPort = parsePort(v[2])
		}
		return ipStr, sshPort, wgPort, nil
	default:
		if val == nil {
			return "", 0, 0, fmt.Errorf("nil output value")
		}
		return "", 0, 0, fmt.Errorf("unsupported output value type %T", val)
	}
}

// parsePort parses a terraform output element into a positive port number,
// returning 0 when it is empty, null, or not a valid port.
func parsePort(val any) int32 {
	p, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(val)))
	if err != nil || p < 1 || p > 65535 {
		return 0
	}
	return int32(p)
}

// readIPs reads json output format from tofu and unmarshal it into map[string]map[string]string readable by Go.
func readIPs(data string) (extofu.NodepoolIPs, error) {
	var result extofu.NodepoolIPs
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
	return result, err
}

// generateProviderTemplates generates only the `provider.tpl` templates so tofu can destroy the infra if needed.
func (c *ClusterBuilder) generateProviderTemplates(directory string, clusterData extofu.ClusterData) error {
	// Need to append also the nodepools that are no longer present in the infrastructure
	// so that their statefile records will get cleaned up.
	nps := append(c.NodePools, c.GhostNodePools...)

	for info, pools := range nodepools.ByProviderDynamic(nps) {
		if err := fileutils.CreateKey(info.Creds, directory, info.SpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", info.SpecName, directory, err)
		}

		templatesDownloadDir := filepath.Join(TemplatesRootDir, c.ClusterId, info.SpecName)
		for _, pools := range extofu.NodePoolsByTemplatesPath(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()
			if err := extofu.Download(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("failed to download template repository for cluster %q provider %q", c.ClusterId, p.SpecName)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			g := extofu.Generator{
				ID:                c.ClusterId,
				TargetDirectory:   directory,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      extofu.TemplatesPath(p),
				Fingerprint:       extofu.Fingerprint(p),
			}

			err := g.GenerateProvider(&extofu.Provider{
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
