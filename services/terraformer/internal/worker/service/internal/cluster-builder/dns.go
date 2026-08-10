package cluster_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/extemplates/extofu"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/tofu"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
)

var (
	// ErrTofuDns is returned when operations related to reconciling
	// DNS resources on the tofu level fail.
	ErrTofuDns = errors.New("dns operation failed")
)

// DnsBuilder aggregates information about a DNS for a loadbalancer
// cluster and provides utility functions for reconciling, deleting
// DNS related resources.
//
// Before using any of the functions, [DnsBuilder.Init] must be called
// with the context of the DNS. After calling all operations the
// [DnsBuilder.Cleanup] function must be called.
type DnsBuilder struct {
	// ClusterName is the name of the loadbalancer cluster in the [DnsBuilder.InputManifest]
	ClusterName string

	// ClusterHash is the hash generated for the [DnsBuilder.ClusterName]
	ClusterHash string

	// ClusterId is the ID of the loadbalancer cluster.
	ClusterId string

	// Inputmanifest is the name of the InputManifest from which the [DnsBuilder.ClusterName] is fro0m
	InputManifest string

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted

	inner struct {
		log    zerolog.Logger
		dns    *spec.DNS
		ips    []string
		dnsDir string
		dnsId  string
	}
}

// Initializes the context for the [DnsBuilder] with the DNS and the desired IPs.
//
// The IPs must be all of the IPs that the DNS records will target. Therefore any
// unwanted, if any, ips must be fileted out before passing into this function.
//
// This function also downloads external templates, if already not present and
// prepares them for the passed in [spec.DNS].
func (b *DnsBuilder) Init(logger zerolog.Logger, ips []string, dns *spec.DNS) error {
	if dns == nil {
		return errors.New("no dns supplied")
	}
	b.inner.log = logger.With().Str("endpoint", dns.Endpoint).Logger()
	b.inner.dns = dns
	b.inner.ips = ips
	b.inner.dnsDir = filepath.Join(Output, fmt.Sprintf("%s-dns", b.ClusterId))
	b.inner.dnsId = fmt.Sprintf("%s-dns", b.ClusterId)

	// Cleanup any previous attempts.
	if err := os.RemoveAll(b.inner.dnsDir); err != nil {
		return fmt.Errorf("failed to cleanup previous work at %q: %w", b.inner.dnsDir, err)
	}

	var err error
	defer func() {
		if err != nil {
			b.Cleanup()
		}
	}()

	if err = b.ensureTemplates(); err != nil {
		return err
	}

	pv, err := b.readProviderVersion()
	if err != nil {
		return err
	}

	if err = b.generateDns(); err != nil {
		return err
	}

	if err = generateProviderVersions(filepath.Join(b.inner.dnsDir, providerVersionFileName), pv); err != nil {
		return err
	}
	return nil
}

func (b *DnsBuilder) Cleanup() {
	if err := os.RemoveAll(b.inner.dnsDir); err != nil {
		b.inner.log.Err(err).Msgf("error while removing files in dir %q: %v", b.inner.dnsDir, err)
	}
}

func (b *DnsBuilder) readProviderVersion() (map[string]ProviderBlock, error) {
	p := b.inner.dns.Provider
	r := filepath.Join(TemplatesRootDir, b.ClusterId, p.SpecName)
	f := filepath.Join(r, extofu.TemplatesProviderVersionPath(p))

	pv, err := parseProviderVersions(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions for dns provider %q inside cluster %q: %w", p.SpecName, b.ClusterId, err)
	}
	return pv, nil
}

func (b *DnsBuilder) ReconcileRecords() error {
	tofu := tofu.Terraform{
		Directory: b.inner.dnsDir,
		CacheDir:  CacheDir,
		Stdout: b.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: b.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: b.SpawnProcessLimit,
	}

	if err := apply(b.inner.log, tofu, b.InputManifest, DnsStateKey(b.ClusterId)); err != nil {
		return fmt.Errorf("%w:%w", ErrTofuDns, err)
	}

	output, err := tofu.OutputString(extofu.DnsEndpointTerraformKey(b.inner.dns, b.ClusterId, ""))
	if err != nil {
		return fmt.Errorf("error while retrieving output after reconciling dns records for %q: %w", b.inner.dnsId, err)
	}

	var result extofu.DNSOutput
	if err := json.Unmarshal([]byte(output), &result.Domain); err != nil {
		return fmt.Errorf("error while retrieving output after reconciling dns records for %q: %w", b.inner.dnsId, err)
	}

	outputID := fmt.Sprintf("%s-endpoint", b.ClusterId)
	b.inner.dns.Endpoint = validateDomain(result.Domain[outputID])
	for _, n := range b.inner.dns.AlternativeNames {
		b.inner.log.
			Info().
			Msgf("Detected alternative names extension, reading output for alternative name %q", n.Hostname)

		if output, err = tofu.OutputString(extofu.DnsEndpointTerraformKey(b.inner.dns, b.ClusterId, n.Hostname)); err != nil {
			// Since this is an extension to the original data we consider errors as not fatal.
			b.inner.log.
				Warn().
				Msgf(
					"error while retrieving output from tofu for %s alternative name %q: %v, templates may not support alternative names extension, skipping",
					b.ClusterId,
					n.Hostname,
					err,
				)

			continue
		}

		if err := json.Unmarshal([]byte(output), &result.Domain); err != nil {
			return fmt.Errorf("error while reading alternative %s name from tofu output for %q: %w", n.Hostname, b.inner.dnsId, err)
		}

		outputID = fmt.Sprintf("%s-%s-endpoint", b.ClusterId, n.Hostname)
		n.Endpoint = validateDomain(result.Domain[outputID])

		b.inner.log.
			Info().
			Msgf("DNS alternative name %q successfully set up", n.Hostname)
	}
	return nil
}

func (b *DnsBuilder) DestroyRecords() error {
	tofu := tofu.Terraform{
		Directory: b.inner.dnsDir,
		CacheDir:  CacheDir,
		Stdout: b.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		Stderr: b.inner.log.Output(zerolog.ConsoleWriter{
			Out:          os.Stderr,
			TimeFormat:   loggerutils.LogTimeFormat,
			FormatLevel:  tofuFormatLevel(),
			FormatCaller: tofuFormatCaller(),
		}),
		SpawnProcessLimit: b.SpawnProcessLimit,
	}

	if err := destroy(b.inner.log, tofu, b.InputManifest, DnsStateKey(b.ClusterId)); err != nil {
		return fmt.Errorf("%w:%w", ErrTofuDns, err)
	}
	return nil
}

func (b *DnsBuilder) ensureTemplates() error {
	p := b.inner.dns.Provider
	d := filepath.Join(TemplatesRootDir, b.ClusterId, p.SpecName)

	if err := extofu.Download(d, p); err != nil {
		return fmt.Errorf("failed to setup template repositor for provider %q inside cluster %q: %w", p.SpecName, b.ClusterId, err)
	}
	return nil
}

func (b *DnsBuilder) generateDns() error {
	p := b.inner.dns.Provider
	t := filepath.Join(TemplatesRootDir, b.ClusterId, p.SpecName)
	g := extofu.Generator{
		ID:                b.inner.dnsId,
		TargetDirectory:   b.inner.dnsDir,
		ReadFromDirectory: t,
		TemplatePath:      extofu.TemplatesPath(p),
		Fingerprint:       extofu.Fingerprint(p),
	}

	data := extofu.DNS{
		DNSZone:     b.inner.dns.DnsZone,
		Hostname:    b.inner.dns.Hostname,
		ClusterName: b.ClusterName,
		ClusterHash: b.ClusterHash,
		RecordData:  extofu.RecordData{IP: templateIPData(b.inner.ips)},
		Provider:    p,

		AlternativeNamesExtension: new(extofu.AlternativeNamesExtension),
		ProviderExtrasExtension:   new(extofu.ProviderExtrasExtension),
	}

	for _, n := range b.inner.dns.AlternativeNames {
		data.AlternativeNamesExtension.Names = append(data.AlternativeNamesExtension.Names, n.Hostname)
	}

	if cloudflare := p.GetCloudflare(); cloudflare != nil {
		var err error
		data.ProviderExtrasExtension.SubscriptionAllowsHA, err = cloudflare.GetSubscription()
		if err != nil {
			if errors.Is(err, spec.ErrCloudflareAPIForbidden) {
				b.inner.log.
					Warn().
					Msgf("Cloudflare API forbidden for provider %q: token/account pair does not have access "+
						"to the subscriptions API, HA loadbalancing will not be used", p.SpecName)
			} else {
				return fmt.Errorf("error while checking cloudflare load balancing subscription: %w", err)
			}
		}
		if !data.ProviderExtrasExtension.SubscriptionAllowsHA {
			b.inner.log.
				Warn().
				Msgf("No Load Balancing subscription found for cloudflare provider %q, HA loadbalancing will not be used", p.SpecName)
		} else {
			b.inner.log.
				Info().
				Msgf("Found subscription for HA load balancing for cloudflare provider %q", p.SpecName)
		}
	}

	if err := g.GenerateDNS(&data); err != nil {
		return err
	}

	if err := fileutils.CreateKey(p.Credentials(), g.TargetDirectory, p.SpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", p.SpecName, g.TargetDirectory, err)
	}
	return nil
}

func templateIPData(ips []string) []extofu.IPData {
	out := make([]extofu.IPData, 0, len(ips))

	for _, ip := range ips {
		out = append(out, extofu.IPData{V4: ip})
	}

	return out
}

// validateDomain validates the domain does not start with ".".
func validateDomain(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}
