package cluster_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	"github.com/hashicorp/go-version"
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

// constraintClauseRegexp splits a single version constraint clause into its
// operator and version. It mimics the unexported constraint regexp used by
// the parseSingle function of [github.com/hashicorp/go-version], built from
// the same exported [version.VersionRegexpRaw] grammar, as the parsed version
// is not accessible through the public API of the [version.Constraint] type.
var constraintClauseRegexp = regexp.MustCompile(fmt.Sprintf(
	`^\s*(%s)\s*(%s)\s*$`,
	`<=|>=|!=|~>|<|>|=|`,
	version.VersionRegexpRaw,
))

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
// operations the [ClusterBuilder.Cleanup] function must be called.
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

		optionalProviders []*spec.Provider
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
//
// The 'optionalProviders' slice of dynamic nodepools is only used to generate providers
// which may be usefull to remove stale infra from the common infrastructure.
func (c *ClusterBuilder) Init(log zerolog.Logger, dynamic []*spec.NodePool, optionalProviders ...*spec.NodePool) error {
	c.inner.log = log
	c.inner.dynamic = dynamic
	c.inner.clusterDir = filepath.Join(Output, c.ClusterId)
	c.inner.networkingDir = filepath.Join(c.inner.clusterDir, NetworkingGenTarget)
	c.inner.nodepoolsDir = filepath.Join(c.inner.clusterDir, NodepoolsGenTarget)

	// Cleanup any previous attemps.
	if err := os.RemoveAll(c.inner.clusterDir); err != nil {
		return fmt.Errorf("failed to cleanup previous work at %q: %w", c.inner.clusterDir, err)
	}

	var err error
	defer func() {
		if err != nil {
			c.Cleanup()
		}
	}()

	optional := make(map[string]map[string][]*spec.NodePool)
	for specName, nps := range nodepools.ByProviderSpecName(optionalProviders) {
		optional[specName] = make(map[string][]*spec.NodePool)
		for path, group := range extofu.NodePoolsByTemplatesVersion(nps) {
			optional[specName][path] = append(optional[specName][path], group...)
		}
	}

	usedProviders := make(map[string]ProviderBlock, len(dynamic))
	for specName, nps := range nodepools.ByProviderSpecName(c.inner.dynamic) {
		for path, group := range extofu.NodePoolsByTemplatesVersion(nps) {
			var providers map[string]ProviderBlock
			if err = ensureTemplates(c.ClusterId, group); err != nil {
				return err
			}
			providers, err = readProviderVersion(c.ClusterId, group)
			if err != nil {
				return err
			}
			if err = mergeProviderVersions(usedProviders, providers, c.inner.log); err != nil {
				return err
			}
			if err = c.generateCommonNetworking(group); err != nil {
				return err
			}
			// For generating the providers include also any
			// deleted optional nodepools to have their
			// providers generate, so that common infrastructure
			// can be cleaned up. Only nodepools of the same
			// templates version are merged in; a different
			// version renders as its own group with its own
			// fingerprint below.
			if o, ok := optional[specName][path]; ok {
				group = append(slices.Clone(group), o...)
				delete(optional[specName], path)
			}
			if err = c.generateCommonNetworkingProviders(group); err != nil {
				return err
			}
		}
	}

	for _, nps := range optional {
		for _, group := range nps {
			var providers map[string]ProviderBlock
			if err = ensureTemplates(c.ClusterId, group); err != nil {
				return err
			}
			providers, err = readProviderVersion(c.ClusterId, group)
			if err != nil {
				return err
			}
			if err = mergeProviderVersions(usedProviders, providers, c.inner.log); err != nil {
				return err
			}
			if err = c.generateCommonNetworkingProviders(group); err != nil {
				return err
			}
		}
	}

	if err = generateProviderVersions(filepath.Join(c.inner.networkingDir, providerVersionFileName), usedProviders); err != nil {
		return err
	}
	return nil
}

func (c *ClusterBuilder) Cleanup() {
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

	if err := apply(c.inner.log, tofu, c.InputManifest, NodePoolStateKey(c.ClusterId, np.Name)); err != nil {
		return fmt.Errorf("%w: %w", ErrTofuNodePool, err)
	}

	output, err := tofu.OutputString(extofu.NodePoolTerraformKey(np))
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

	if err := destroy(c.inner.log, tofu, c.InputManifest, NodePoolStateKey(c.ClusterId, np.Name)); err != nil {
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

	if err := tinit(c.inner.log, tofu, c.InputManifest, CommonInfraStateKey(c.ClusterId)); err != nil {
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

	if err := apply(c.inner.log, tofu, c.InputManifest, CommonInfraStateKey(c.ClusterId)); err != nil {
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

	if err := destroy(c.inner.log, tofu, c.InputManifest, CommonInfraStateKey(c.ClusterId)); err != nil {
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

func ensureTemplates(clusterId string, sameProviderGroup []*spec.NodePool) error {
	p := sameProviderGroup[0].GetDynamicNodePool().Provider
	d := filepath.Join(TemplatesRootDir, clusterId, p.SpecName)

	// Validation guarantees that the specName is unique within a single InputManifest,
	// thus when we group nodepools by specName they all point to the same provider
	// and we can download the templates for the whole group with just a single call.
	if err := extofu.Download(d, p); err != nil {
		return fmt.Errorf("failed to setup template repository for provider %q inside cluster %q: %w", p.SpecName, clusterId, err)
	}
	return nil
}

// readProviderVersion reads the provider requirements pinned by the external templates of the provider.
func readProviderVersion(clusterId string, sameProviderGroup []*spec.NodePool) (map[string]ProviderBlock, error) {
	p := sameProviderGroup[0].GetDynamicNodePool().Provider
	r := filepath.Join(TemplatesRootDir, clusterId, p.SpecName)
	f := filepath.Join(r, extofu.TemplatesProviderVersionPath(p))

	pv, err := parseProviderVersions(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions for provider %q inside cluster %q: %w", p.SpecName, clusterId, err)
	}
	return pv, nil
}

// generateCommonNetworkingProviders generates the providers used within the `networking` folder
// of the external templates for spawning common networking infrastructure for all of the nodepools.
func (c *ClusterBuilder) generateCommonNetworkingProviders(sameProviderGroup []*spec.NodePool) error {
	p := sameProviderGroup[0].GetDynamicNodePool().Provider
	d := nodepools.ExtractDynamic(sameProviderGroup)
	r := nodepools.ExtractRegions(d)
	var rgn []extofu.RegionNetwork

	for _, v := range nodepools.ExtractRegionNetwork(d) {
		rgn = append(rgn, extofu.RegionNetwork(v))
	}

	g := extofu.Generator{
		ID:                c.ClusterId,
		TargetDirectory:   c.inner.networkingDir,
		ReadFromDirectory: filepath.Join(TemplatesRootDir, c.ClusterId, p.SpecName),
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

	if err := g.GenerateNetworkingProvider(&n); err != nil {
		return fmt.Errorf("failed to generate networking module for provider %q in cluster %q: %w", p.SpecName, c.ClusterId, err)
	}
	if err := fileutils.CreateKey(p.Credentials(), c.inner.networkingDir, p.SpecName); err != nil {
		return fmt.Errorf("error generating credentials file used for networking for cluster %q: %w", c.ClusterId, err)
	}
	return nil
}

// generateCommonNetworking generates the 'networking' folder of the external templates for
// spawning common networking infrastructure for all the nodepools.
func (c *ClusterBuilder) generateCommonNetworking(sameProviderGroup []*spec.NodePool) error {
	p := sameProviderGroup[0].GetDynamicNodePool().Provider
	d := nodepools.ExtractDynamic(sameProviderGroup)
	r := nodepools.ExtractRegions(d)
	var rgn []extofu.RegionNetwork

	for _, v := range nodepools.ExtractRegionNetwork(d) {
		rgn = append(rgn, extofu.RegionNetwork(v))
	}

	g := extofu.Generator{
		ID:                c.ClusterId,
		TargetDirectory:   c.inner.networkingDir,
		ReadFromDirectory: filepath.Join(TemplatesRootDir, c.ClusterId, p.SpecName),
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
	return nil
}

// generateNodePool generates a single nodepool using the external nodepools to be used with tofu.
func (c *ClusterBuilder) generateNodePool(np *spec.NodePool, out extofu.NetworkingOutput) error {
	d := filepath.Join(c.inner.nodepoolsDir, np.Name)
	p := np.GetDynamicNodePool().Provider
	g := extofu.Generator{
		ID:                c.ClusterId,
		TargetDirectory:   d,
		ReadFromDirectory: filepath.Join(TemplatesRootDir, c.ClusterId, p.SpecName),
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
		return fmt.Errorf("error generating public key for %s: %w", np.Name, err)
	}
	if err := fileutils.CreateKey(p.Credentials(), d, p.SpecName); err != nil {
		return fmt.Errorf("error generating credentials file for %q: %w", np.Name, err)
	}

	v, err := readProviderVersion(c.ClusterId, []*spec.NodePool{np})
	if err != nil {
		return err
	}

	return generateProviderVersions(filepath.Join(g.TargetDirectory, providerVersionFileName), v)
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
