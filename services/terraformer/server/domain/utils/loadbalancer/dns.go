package loadbalancer

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/server/domain/utils/cluster-builder"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/tofu"
	"github.com/rs/zerolog"

	"google.golang.org/protobuf/proto"

	"golang.org/x/sync/semaphore"
)

const (
	TemplatesRootDir = "services/terraformer/templates"
)

type DNS struct {
	ProjectName string
	ClusterName string
	ClusterHash string

	DesiredNodeIPs []string
	CurrentNodeIPs []string

	CurrentDNS *spec.DNS
	DesiredDNS *spec.DNS

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
	// Role holds port number and protocol, which will be used to create health checks for the DNS records.
	Role *spec.Role
}

// CreateDNSRecords creates DNS records for the Loadbalancer cluster.
func (d *DNS) CreateDNSRecords(logger zerolog.Logger) error {
	sublogger := logger.With().Str("endpoint", d.DesiredDNS.Endpoint).Logger()

	clusterID := fmt.Sprintf("%s-%s", d.ClusterName, d.ClusterHash)
	dnsID := fmt.Sprintf("%s-dns", clusterID)
	dnsDir := filepath.Join(cluster_builder.Output, dnsID)

	tofu := tofu.Terraform{
		Directory:         dnsDir,
		SpawnProcessLimit: d.SpawnProcessLimit,
	}

	tofu.Stdout = comm.GetStdOut(clusterID)
	tofu.Stderr = comm.GetStdErr(clusterID)

	defer func() {
		if err := os.RemoveAll(dnsDir); err != nil {
			sublogger.Err(err).Msgf("error while removing files in dir %q: %v", dnsDir, err)
		}
	}()

	if changedDNSProvider(d.CurrentDNS, d.DesiredDNS) {
		sublogger.Info().Msg("Destroying old DNS records")
		if err := d.generateProvider(dnsID, dnsDir, d.CurrentDNS, d.DesiredDNS); err != nil {
			return fmt.Errorf("error while generating providers tf files for %s: %w", dnsID, err)
		}
		// destroy current state.
		if err := d.generateFiles(dnsID, dnsDir, d.CurrentDNS, d.CurrentNodeIPs); err != nil {
			return fmt.Errorf("error while creating current state dns.tf files for %s : %w", dnsID, err)
		}
		// In case of a re-execution of a task which would fail, if we do not
		// delete also the desired state, which might have been created.
		if err := d.generateFiles(dnsID, dnsDir, d.DesiredDNS, d.DesiredNodeIPs); err != nil {
			return fmt.Errorf("error while creating desired state dns.tf files for %s : %w", dnsID, err)
		}
		if err := tofu.Init(); err != nil {
			return err
		}

		stateFile, err := tofu.StateList()
		if err != nil {
			sublogger.Warn().Msgf("absent statefile for dns, assumming the previous state was not build correctly")
		}

		if err := tofu.DestroyTarget(stateFile); err != nil {
			return fmt.Errorf("failed to destroy existing DNS state: %w", err)
		}

		if err := os.RemoveAll(dnsDir); err != nil {
			return fmt.Errorf("error while removing files in dir %q: %w", dnsDir, err)
		}

		sublogger.Info().Msg("Old DNS records were destroyed")
	}

	sublogger.Info().Msg("Creating new DNS records")

	if err := d.generateProvider(dnsID, dnsDir, nil, d.DesiredDNS); err != nil {
		return fmt.Errorf("error while generating providers tf files for %s: %w", dnsID, err)
	}

	if err := d.generateFiles(dnsID, dnsDir, d.DesiredDNS, d.DesiredNodeIPs); err != nil {
		return fmt.Errorf("error while creating dns .tf files for %s : %w", dnsID, err)
	}

	if err := tofu.Init(); err != nil {
		return err
	}

	var currentState []string
	if d.CurrentDNS != nil {
		var err error
		if currentState, err = tofu.StateList(); err != nil {
			return fmt.Errorf("error while retreiving tofu state list, before applying changes in %s: %w", dnsID, err)
		}
	}

	if err := tofu.Apply(); err != nil {
		updatedState, errList := tofu.StateList()
		if errList != nil {
			return errors.Join(err, fmt.Errorf("%w: error while retrieving tofu state list after applying changes in %s: %w", err, dnsID, errList))
		}

		var toDelete []string
		for _, resource := range updatedState {
			if !slices.Contains(currentState, resource) {
				toDelete = append(toDelete, resource)
			}
		}

		if errDestroy := tofu.DestroyTarget(toDelete); errDestroy != nil {
			return fmt.Errorf("%w: failed to destroy partially created state: %w", err, errDestroy)
		}

		return err
	}

	output, err := tofu.Output(endpoint(d.DesiredDNS, clusterID, ""))
	if err != nil {
		return fmt.Errorf("error while getting output from tofu for %s : %w", dnsID, err)
	}

	out, err := readDomain(output)
	if err != nil {
		return fmt.Errorf("error while reading output from tofu for %s : %w", dnsID, err)
	}

	outputID := fmt.Sprintf("%s-endpoint", clusterID)
	sublogger.Info().Msg("DNS records were successfully set up")

	d.DesiredDNS.Endpoint = validateDomain(out.Domain[outputID])

	for _, n := range d.DesiredDNS.AlternativeNames {
		sublogger.Info().Msgf("Detected alternative names extension, reading output for alternative name %s", n.Hostname)

		if output, err = tofu.Output(endpoint(d.DesiredDNS, clusterID, n.Hostname)); err != nil {
			// Since this is an extension to the original data
			// we consider errors as not fatal.
			sublogger.Warn().Msgf("error while retrieving output from tofu for %s alternative name %s: %v, templates may not support alternative names extension, skipping", clusterID, n.Hostname, err)
			continue
		}

		if out, err = readDomain(output); err != nil {
			return fmt.Errorf("error while reading alternative %s name from tofu output for %s: %w, skipping", n.Hostname, dnsID, err)
		}

		outputID = fmt.Sprintf("%s-%s-endpoint", clusterID, n.Hostname)
		n.Endpoint = validateDomain(out.Domain[outputID])
		sublogger.Info().Msg("DNS alternative name successfully set up")
	}

	return nil
}

// DestroyDNSRecords destroys DNS records for the Loadbalancer cluster.
func (d *DNS) DestroyDNSRecords(logger zerolog.Logger) error {
	sublogger := logger.With().Str("endpoint", d.CurrentDNS.Endpoint).Logger()

	sublogger.Info().Msg("Destroying DNS records")
	dnsID := fmt.Sprintf("%s-%s-dns", d.ClusterName, d.ClusterHash)
	dnsDir := filepath.Join(cluster_builder.Output, dnsID)

	defer func() {
		if err := os.RemoveAll(dnsDir); err != nil {
			sublogger.Err(err).Msgf("error while removing files in dir %q: %v", dnsDir, err)
		}
	}()

	if err := d.generateProvider(dnsID, dnsDir, d.CurrentDNS, nil); err != nil {
		return fmt.Errorf("error while generating providers tf files for %s: %w", dnsID, err)
	}

	if err := d.generateFiles(dnsID, dnsDir, d.CurrentDNS, d.CurrentNodeIPs); err != nil {
		return fmt.Errorf("error while creating dns records for %s : %w", dnsID, err)
	}

	tofu := tofu.Terraform{
		Directory:         dnsDir,
		SpawnProcessLimit: d.SpawnProcessLimit,
	}

	tofu.Stdout = comm.GetStdOut(dnsID)
	tofu.Stderr = comm.GetStdErr(dnsID)

	if err := tofu.Init(); err != nil {
		return err
	}

	if err := tofu.Destroy(); err != nil {
		return err
	}

	sublogger.Info().Msg("DNS records were successfully destroyed")

	return nil
}

func (d *DNS) generateProvider(dnsID, dnsDir string, current, desired *spec.DNS) error {
	backend := templates.Backend{
		ProjectName: d.ProjectName,
		ClusterName: dnsID,
		Directory:   dnsDir,
	}

	if err := backend.CreateTFFile(); err != nil {
		return err
	}

	usedProviders := templates.UsedProviders{
		ProjectName: d.ProjectName,
		ClusterName: dnsID,
		Directory:   dnsDir,
	}

	return usedProviders.CreateUsedProviderDNS(current, desired)
}

// generateFiles creates all the necessary terraform files used to create/destroy DNS.
func (d *DNS) generateFiles(dnsID, dnsDir string, dns *spec.DNS, nodeIPs []string) error {
	templateDir := filepath.Join(TemplatesRootDir, dnsID, dns.GetProvider().GetSpecName())
	if err := templates.DownloadProvider(templateDir, dns.GetProvider()); err != nil {
		return fmt.Errorf("failed to download templates for DNS %q: %w", dnsID, err)
	}

	path := dns.Provider.Templates.MustExtractTargetPath()

	g := templates.Generator{
		ID:                dnsID,
		TargetDirectory:   dnsDir,
		ReadFromDirectory: templateDir,
		TemplatePath:      path,
		Fingerprint:       hex.EncodeToString(hash.Digest128(filepath.Join(dns.Provider.SpecName, path))),
	}

	var cloudflareSubscription bool
	if dns.Provider.GetCloudProviderName() == "cloudflare" {
		var err error
		cloudflareSubscription, err = getCloudflareSubscription(dns.Provider.GetCloudflare().GetToken(), dns.GetHostname())
		if err != nil {
			return fmt.Errorf("Error while checking cloudflare Load Balancing subscription %w", err)
		}
	}

	// string AccountId = dns.Provider.GetCloudflare().GetAccountId()
	data := templates.DNS{
		DNSZone:                dns.DnsZone,
		Hostname:               dns.Hostname,
		ClusterName:            d.ClusterName,
		ClusterHash:            d.ClusterHash,
		RecordData:             templates.RecordData{IP: templateIPData(nodeIPs)},
		Provider:               dns.Provider,
		Role:                   d.Role,
		CloudflareSubscription: cloudflareSubscription,

		AlternativeNamesExtension: new(templates.AlternativeNamesExtension),
	}

	for _, n := range dns.AlternativeNames {
		data.AlternativeNamesExtension.Names = append(data.AlternativeNamesExtension.Names, n.Hostname)
	}

	if err := g.GenerateDNS(&data); err != nil {
		return fmt.Errorf("failed to generate dns templates for %q: %w", dnsID, err)
	}

	if err := fileutils.CreateKey(data.Provider.Credentials(), g.TargetDirectory, data.Provider.SpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", data.Provider.SpecName, g.TargetDirectory, err)
	}

	return nil
}

func getCloudflareSubscription(apiToken string, zoneName string) (bool, error) {

	var accountID string

	var subscriptions struct {
		Result []struct {
			ID      string `json:"id"`
			Product struct {
				Name string `json:"name"`
			} `json:"product"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	urlSubscriptions := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts")

	responseSubscriptions, err := getCloudflareAPIResponse(urlSubscriptions, apiToken)

	if err != nil {
		return false, fmt.Errorf("error while getting cloudflare api response for %s: %w", urlSubscriptions, err)
	}

	if err := json.Unmarshal(responseSubscriptions, &subscriptions); err != nil {
		return false, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	for _, subscription := range subscriptions.Result {
		if subscription.Product.Name == "prod_load_balancing" && subscriptions.Success == true {
			fmt.Errorf("Found subscription for %s\n", subscription.Product.Name)
			return true, nil
		}
	}
	return false, fmt.Errorf("Subscription for Load Balancing not found")
}

func getCloudflareAPIResponse(url string, apiToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not create request: %s", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("Error making http request: %s", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %s", err)
	}

	defer resp.Body.Close()

	return body, nil

}

// validateDomain validates the domain does not start with ".".
func validateDomain(s string) string {
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

// readDomain reads full domain from tofu output.
func readDomain(data string) (templates.DNSDomain, error) {
	var result templates.DNSDomain
	err := json.Unmarshal([]byte(data), &result.Domain)
	return result, err
}

func changedDNSProvider(currentDNS, desiredDNS *spec.DNS) bool {
	// DNS not yet created
	if currentDNS == nil {
		return false
	}
	// DNS provider are same
	if currentDNS.Provider.SpecName == desiredDNS.Provider.SpecName {
		if proto.Equal(currentDNS.Provider, desiredDNS.Provider) {
			return false
		}
	}
	return true
}

func templateIPData(ips []string) []templates.IPData {
	out := make([]templates.IPData, 0, len(ips))

	for _, ip := range ips {
		out = append(out, templates.IPData{V4: ip})
	}

	return out
}

func endpoint(dns *spec.DNS, clusterID string, alternativeName string) string {
	f := hash.Digest128(filepath.Join(
		dns.GetProvider().GetSpecName(),
		dns.GetProvider().GetTemplates().MustExtractTargetPath(),
	))
	resourceSuffix := fmt.Sprintf("%s_%s", dns.GetProvider().GetSpecName(), hex.EncodeToString(f))
	resource := clusterID
	if alternativeName != "" {
		resource += "_" + alternativeName
	}
	return fmt.Sprintf("%s_%s", resource, resourceSuffix)
}
