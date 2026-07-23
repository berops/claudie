package cluster_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/berops/claudie/internal/extemplates/extofu"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/generics"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/templates"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/tofu"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

var (
	// ErrTofuNodePool is returned when operations related to reconciling/destroying nodepools on the tofu level fail.
	ErrTofuNodePool = errors.New("nodepool operation failed")

	// ErrTofuCommonInfrastructure is returned when operations related to reconciling/destroying infrastructure
	// common to all nodepools on the tofu level fail.
	ErrTofuCommonInfrastructure = errors.New("common infrastructure operation failed")
)

// KubernetesClusterInfo provides additional information that is
// specific to a kuberentes cluster when using [ClusterBuilder]
type KubernetesClusterInfo struct {
	ExportPort6443 bool
}

// LoadbalancerClusterInfo providers additional information that is
// specific to a loadbalancer cluster when using [ClusterBuilder]
type LoadbalancerClusterInfo struct {
	Roles []*spec.Role
}

// ClusterBuilder aggregates information about a cluster and provides
// utility functions for reconciling, building, deleting clusters and
// their resources.
//
// Before using any of the functions, [ClusterBuilder.Init] must be called
// with the context of the nodepools for the cluster. After calling all
// operations the [ClusterBuilder.Done] function must be called.
type ClusterBuilder struct {
	// ClusterName of the cluster, as stated in the [ClusterBuilder.InputManifest].
	ClusterName string

	// ClusterHash that was generated for the [ClusterBuilder.ClusterName]
	ClusterHash string

	// ClusterId of the cluster.
	ClusterId string

	// InputManifest is the name of the InputManifest from which the [ClusterBuilder.ClusterName] is from.
	InputManifest string

	// Type of the cluster
	Type ClusterType

	// K8sInfo is additional data for when [ClusterBuilder.Type] is of [KubernetesCluster]
	K8sInfo KubernetesClusterInfo

	// LBInfo is additional data for when [ClusterBuilder.Type] is of [LoadbalancerCluster]
	LBInfo LoadbalancerClusterInfo

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted

	inner struct {
		log           zerolog.Logger
		dynamic       []*spec.NodePool
		clusterDir    string
		networkingDir string
		nodepoolsDir  string
	}
}

// Initializes the context for the [ClusterBuilder] with the nodepools.
//
// The nodepools must be dynamic only nodepools. It is therefore necessary
// to filter out non-dynamic nodes before initializing [ClusterBuilder].
//
// This function also downloads external templates, if already not present
// and prepares the infrastructure shared among all of the nodepools.
//
// It is important that the passed in nodepools reflect the actual to be reconciled
// state of the cluster.
func (c *ClusterBuilder) Init(log zerolog.Logger, dynamic []*spec.NodePool) error {
	c.inner.log = log
	c.inner.dynamic = dynamic
	c.inner.clusterDir = filepath.Join(Output, c.ClusterId)
	c.inner.networkingDir = filepath.Join(c.inner.clusterDir, NetworkingGenTarget)
	c.inner.nodepoolsDir = filepath.Join(c.inner.clusterDir, NodepoolsGenTarget)

	for _, group := range nodepools.ByProviderSpecName(c.inner.dynamic) {
		if err := c.ensureTemplates(group); err != nil {
			return err
		}
		if err := c.generateCommonNetworking(group); err != nil {
			return err
		}
	}

	return nil
}

func (c *ClusterBuilder) Done() {
	if err := os.RemoveAll(c.inner.clusterDir); err != nil {
		c.inner.log.Err(err).Msgf("error when deleting generated files in %s: %v", c.inner.clusterDir, err)
	}
}

func (c *ClusterBuilder) ReconcileNodePool(nto extofu.NetworkingOutput, handle int) error {
	np := c.inner.dynamic[handle]
	if err := c.generateNodePool(np, nto); err != nil {
		return err
	}

	tofu := tofu.Terraform{
		Directory: filepath.Join(c.inner.nodepoolsDir, np.Name),
		CacheDir:  CacheDir,
		Stdout: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	if err := apply(c.inner.log, tofu, c.InputManifest, StateFileNodePoolSubKey(c.ClusterId, np.Name)); err != nil {
		return fmt.Errorf("%w: %w", ErrTofuNodePool, err)
	}

	output, err := tofu.Output(extofu.NodePoolTerraformKey(np))
	if err != nil {
		return err
	}

	var npo extofu.NodepoolOutput
	if err := json.Unmarshal([]byte(output), &npo.IPs); err != nil {
		return fmt.Errorf("failed to read ips from output for nodepool %q: %w", np.Name, err)
	}

	for _, n := range np.Nodes {
		var found bool
		for target, val := range generics.IterateMapInOrder(npo.IPs) {
			if target != n.Name {
				continue
			}
			ip, sshPort, wgPort, err := parseNodeOutput(val)
			if err != nil {
				return fmt.Errorf("node %q from nodepool %q: %w", n.Name, np.Name, err)
			}
			if ip == "" {
				return fmt.Errorf("node %q from nodepool %q has no public address assigned", n.Name, np.Name)
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
			return fmt.Errorf("node %s from nodepool %s was missing from the tofu output, possibly the VM was not properly created", n.Name, np.Name)
		}
	}
	return nil
}

// Destroys the nodepool stored at position 'handle' from the nodepools that were passed in via the [ClusterBuilder.Init] function.
func (c *ClusterBuilder) DestroyNodePool(nto extofu.NetworkingOutput, handle int) error {
	np := c.inner.dynamic[handle]
	if err := c.generateNodePool(np, nto); err != nil {
		return err
	}

	tofu := tofu.Terraform{
		Directory: filepath.Join(c.inner.nodepoolsDir, np.Name),
		CacheDir:  CacheDir,
		Stdout: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	if err := destroy(c.inner.log, tofu, c.InputManifest, StateFileNodePoolSubKey(c.ClusterId, np.Name)); err != nil {
		return fmt.Errorf("%w: %w", ErrTofuNodePool, err)
	}
	return nil
}

func (c *ClusterBuilder) OutputOnlyCommon() (extofu.NetworkingOutput, error) {
	tofu := tofu.Terraform{
		Directory: c.inner.networkingDir,
		CacheDir:  CacheDir,
		Stdout: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	if err := tinit(c.inner.log, tofu, c.InputManifest, StateFileCommonInfrastructureSubKey(c.ClusterId)); err != nil {
		return extofu.NetworkingOutput{}, fmt.Errorf("%w: %w", ErrTofuCommonInfrastructure, err)
	}

	out, err := tofu.OutputAll()
	if err != nil {
		return extofu.NetworkingOutput{}, err
	}
	return extofu.NetworkingOutput{All: out}, nil
}

func (c *ClusterBuilder) ReconcileCommon() (extofu.NetworkingOutput, error) {
	tofu := tofu.Terraform{
		Directory: c.inner.networkingDir,
		CacheDir:  CacheDir,
		Stdout: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	if err := apply(c.inner.log, tofu, c.InputManifest, StateFileCommonInfrastructureSubKey(c.ClusterId)); err != nil {
		return extofu.NetworkingOutput{}, fmt.Errorf("%w: %w", ErrTofuCommonInfrastructure, err)
	}

	out, err := tofu.OutputAll()
	if err != nil {
		return extofu.NetworkingOutput{}, err
	}
	return extofu.NetworkingOutput{All: out}, nil
}

func (c *ClusterBuilder) DestroyCommon() error {
	tofu := tofu.Terraform{
		Directory: c.inner.networkingDir,
		CacheDir:  CacheDir,
		Stdout: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: c.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: c.SpawnProcessLimit,
	}

	if err := destroy(c.inner.log, tofu, c.InputManifest, StateFileCommonInfrastructureSubKey(c.ClusterId)); err != nil {
		return fmt.Errorf("%w: %w", ErrTofuCommonInfrastructure, err)
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
	// parsePort parses a terraform output element into a positive port number,
	// returning 0 when it is empty, null, or not a valid port.
	parsePort := func(val any) int32 {
		p, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(val)))
		if err != nil || p < 1 || p > 65535 {
			return 0
		}
		return int32(p)
	}
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

func (c *ClusterBuilder) ensureTemplates(sameProviderGroup []*spec.NodePool) error {
	p := sameProviderGroup[0].GetDynamicNodePool().Provider
	d := filepath.Join(TemplatesRootDir, c.ClusterId, p.SpecName)

	// Validation guarantees that the specName is unique within a single InputManifest,
	// thus when we group nodepools by specName they all point to the same provider
	// and we can download the templates for the whole group with just a single call.
	if err := extofu.Download(d, p); err != nil {
		return fmt.Errorf("failed to setup template repository for provider %q inside cluster %q: %w", p.SpecName, c.ClusterId, err)
	}
	return nil
}

func (c *ClusterBuilder) generateProviderVersioning(todo) {
	/*
				 TODO:
				 Check the todo in the manager service adding k8s nodepool terraform scheduling if its needed.

				 For The versioning to work we need to check for the `provider_version.tpl` file in the external
				 templates, If present we need to parse it via the HCL into a go struct. see hetzner example in your git repo.


				Do this for every nodepool. Since a cluster can have many providers of the same type
				if there are any version mismatches always take the highest version out of the available
				verions.

				Since each nodepool has its own terraform file this shouldn't be an issue for that.
				The only issue might be caused by the common networking infrastructure if the newer
				version introduces backwards incompatible changes but that shouldn't be an issue
				as tofu will fail and claudie will revert back the changes and loop until the
				errornous template is removed.

				On empty/missing provider_version.tpl error out.


				Might also need to add tofu init -upgrade instead of having plain tofu init.

				Idea something along this way, but for each provider of the same type choose
				the highest version

				Final plan summary:
				File to create:
				templates/terraformer/hetzner/provider.tpl
				Content:
				terraform {
		          required_providers {
		            hcloud = {
		              source  = "hetznercloud/hcloud"
		              version = "~> 1.45"
		            }
		          }
				}
				Claudie aggregation logic (conceptual Go):

				type ProviderReq struct {
		          Source  string `hcl:"source"`
		          Version string `hcl:"version"`
				}

				type TerraformBlock struct {
		          RequiredProviders map[string]ProviderReq `hcl:"required_providers,block"`
				}

				type RootConfig struct {
		          Terraform *TerraformBlock `hcl:"terraform,block"`
				}

				func aggregate(templateDirs []string) (string, error) {
		        all := map[string][]ProviderReq{}                 // name -> versions
		        for _, dir := range templateDirs {
		         path := filepath.Join(dir, "provider.tpl")
		         var cfg RootConfig
		         hclsimple.DecodeFile(path, nil, &cfg)
		         for name, p := range cfg.Terraform.RequiredProviders {
		            all[name] = append(all[name], p)
		         }
		        }

		    // For each provider, check conflicts, pick highest
		    // Use hashicorp/go-version to compare
		    // Emit warning if different. Generate final HCL.
			}
			Strategy: Take highest version per provider, warn on disagreement.
	*/
}

// GenerateCommonNetworking generates the 'networking' folder of the external templates for
// spawning common networking infrastructure for all the nodepools.
func (c *ClusterBuilder) generateCommonNetworking(sameProviderGroup []*spec.NodePool) error {
	p := sameProviderGroup[0].GetDynamicNodePool().Provider
	t := filepath.Join(TemplatesRootDir, c.ClusterId, p.SpecName)
	d := nodepools.ExtractDynamic(sameProviderGroup)
	r := nodepools.ExtractRegions(d)
	var rgn []extofu.RegionNetwork

	for _, v := range nodepools.ExtractRegionNetwork(d) {
		rgn = append(rgn, extofu.RegionNetwork(v))
	}

	g := extofu.Generator{
		ID:                c.ClusterId,
		TargetDirectory:   c.inner.networkingDir,
		ReadFromDirectory: t,
		TemplatePath:      extofu.TemplatesPath(p),
		Fingerprint:       extofu.Fingerprint(p),
	}
	n := extofu.Networking{
		ClusterData: extofu.ClusterData{
			ClusterName: c.ClusterName,
			ClusterHash: c.ClusterHash,
			ClusterType: string(c.Type),
		},
		Provider:      p,
		Regions:       r,
		RegionNetwork: rgn,
		K8sData: extofu.K8sData{
			HasAPIServer: c.K8sInfo.ExportPort6443,
		},
		LBData: extofu.LBData{
			Roles: c.LBInfo.Roles,
		},
	}

	if err := g.GenerateNetworking(&n); err != nil {
		return fmt.Errorf("failed to generate networking module for provider %q in cluster %q: %w", p.SpecName, c.ClusterId, err)
	}
	if err := fileutils.CreateKey(p.Credentials(), c.inner.networkingDir, p.SpecName); err != nil {
		return fmt.Errorf("error generating credentials file used for networking for cluster %q: %w", c.ClusterId, err)
	}
	return nil
}

// generateNodePool generates a single nodepool using the external nodepools to be used with tofu.
func (c *ClusterBuilder) generateNodePool(np *spec.NodePool, out extofu.NetworkingOutput) error {
	d := filepath.Join(c.inner.nodepoolsDir, np.Name)
	p := np.GetDynamicNodePool().Provider
	t := filepath.Join(TemplatesRootDir, c.ClusterId, p.SpecName)
	g := extofu.Generator{
		ID:                c.ClusterId,
		TargetDirectory:   d,
		ReadFromDirectory: t,
		TemplatePath:      extofu.TemplatesPath(p),
		Fingerprint:       extofu.Fingerprint(p),
	}
	n := extofu.Nodepool{
		ClusterData: extofu.ClusterData{
			ClusterName: c.ClusterName,
			ClusterHash: c.ClusterHash,
			ClusterType: string(c.Type),
		},
		NodePool: extofu.NodePoolInfo{
			Name:      np.Name,
			Details:   np.GetDynamicNodePool(),
			Nodes:     np.Nodes,
			IsControl: np.IsControl,
		},
		Networking: out,
	}
	if err := g.GenerateNodes(&n); err != nil {
		return err
	}
	if err := fileutils.CreateKey(np.GetDynamicNodePool().GetPublicKey(), d, np.GetName()); err != nil {
		return fmt.Errorf("error generating public key for %s in cluster %q: %w", np.Name, c.ClusterId, err)
	}
	if err := fileutils.CreateKey(p.Credentials(), d, p.SpecName); err != nil {
		return fmt.Errorf("error generating credentials file for %q in cluster %q: %w", np.Name, c.ClusterId, err)
	}
	return nil
}

func apply(log zerolog.Logger, tofu tofu.Terraform, inputManifest, stateFileKey string) error {
	if err := tinit(log, tofu, inputManifest, stateFileKey); err != nil {
		return err
	}
	return tofu.Apply()
}

func destroy(log zerolog.Logger, tofu tofu.Terraform, inputManifest, stateFileKey string) error {
	if err := tinit(log, tofu, inputManifest, stateFileKey); err != nil {
		return err
	}
	return tofu.Destroy()
}

func tinit(log zerolog.Logger, tofu tofu.Terraform, inputManifest, stateFileKey string) error {
	backend := templates.Backend{
		ProjectName: inputManifest,
		Target:      stateFileKey,
		Directory:   tofu.Directory,
	}

	if err := backend.CreateTFFile(); err != nil {
		return err
	}

	if err := tofu.ProvidersLock(); err != nil {
		log.Warn().Msgf("Error while locking providers from local FS mirror\n" +
			"Continue to retrieve providers and generate hash from remote registry.")
	}

	return tofu.Init()
}
