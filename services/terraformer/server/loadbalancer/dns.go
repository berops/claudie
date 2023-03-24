package loadbalancer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/services/terraformer/server/provider"

	comm "github.com/berops/claudie/internal/command"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/backend"
	"github.com/berops/claudie/services/terraformer/server/clusterBuilder"
	"github.com/berops/claudie/services/terraformer/server/terraform"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type DNS struct {
	ClusterName    string
	ClusterHash    string
	DesiredNodeIPs []string
	CurrentNodeIPs []string
	CurrentDNS     *pb.DNS
	DesiredDNS     *pb.DNS
	ProjectName    string
}

type DNSData struct {
	ClusterName  string
	ClusterHash  string
	HostnameHash string
	DNSZone      string
	NodeIPs      []string
	Provider     *pb.Provider
}

type outputDomain struct {
	Domain map[string]string `json:"-"`
}

func (d DNS) CreateDNSRecords() (string, error) {
	clusterID := fmt.Sprintf("%s-%s", d.ClusterName, d.ClusterHash)
	dnsID := fmt.Sprintf("%s-dns", clusterID)
	dnsDir := filepath.Join(clusterBuilder.Output, dnsID)

	terraform := terraform.Terraform{
		Directory: dnsDir,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		terraform.Stdout = comm.GetStdOut(clusterID)
		terraform.Stderr = comm.GetStdErr(clusterID)
	}

	if utils.ChangedDNSProvider(d.CurrentDNS, d.DesiredDNS) {
		log.Info().Msgf("Destroying old DNS records for %s from cluster %s", d.CurrentDNS.Endpoint, d.ClusterName)
		if err := d.generateFiles(dnsID, dnsDir, d.CurrentDNS, d.CurrentNodeIPs); err != nil {
			return "", fmt.Errorf("error while creating dns .tf files for %s : %w", dnsID, err)
		}
		if err := terraform.TerraformInit(); err != nil {
			return "", err
		}
		if err := terraform.TerraformDestroy(); err != nil {
			return "", err
		}

		if err := os.RemoveAll(dnsDir); err != nil {
			return "", fmt.Errorf("error while removing files in dir %q: %w", dnsDir, err)
		}
	}

	log.Info().Msgf("Creating new DNS records for %s from cluster %s", d.DesiredDNS.Endpoint, d.ClusterName)
	if err := d.generateFiles(dnsID, dnsDir, d.DesiredDNS, d.DesiredNodeIPs); err != nil {
		return "", fmt.Errorf("error while creating dns .tf files for %s : %w", dnsID, err)
	}
	if err := terraform.TerraformInit(); err != nil {
		return "", err
	}
	if err := terraform.TerraformApply(); err != nil {
		return "", err
	}

	outputID := fmt.Sprintf("%s-%s", clusterID, "endpoint")
	output, err := terraform.TerraformOutput(clusterID)
	if err != nil {
		return "", fmt.Errorf("error while getting output from terraform for %s : %w", clusterID, err)
	}

	out, err := readDomain(output)
	if err != nil {
		return "", fmt.Errorf("error while reading output from terraform for %s : %w", clusterID, err)
	}

	log.Info().Msgf("DNS records for %s from cluster %s were successfully set up", d.DesiredDNS.Endpoint, d.ClusterName)
	if err := os.RemoveAll(dnsDir); err != nil {
		return validateDomain(out.Domain[outputID]), fmt.Errorf("error while deleting files in %s: %w", dnsDir, err)
	}

	return validateDomain(out.Domain[outputID]), nil
}

func (d DNS) DestroyDNSRecords() error {
	log.Info().Msgf("Destroying DNS records for %s from cluster %s", d.CurrentDNS.Endpoint, d.ClusterName)
	dnsID := fmt.Sprintf("%s-%s-dns", d.ClusterName, d.ClusterHash)
	dnsDir := filepath.Join(clusterBuilder.Output, dnsID)

	if err := d.generateFiles(dnsID, dnsDir, d.CurrentDNS, d.CurrentNodeIPs); err != nil {
		return fmt.Errorf("error while creating dns records for %s : %w", dnsID, err)
	}

	terraform := terraform.Terraform{
		Directory: dnsDir,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		terraform.Stdout = comm.GetStdOut(dnsID)
		terraform.Stderr = comm.GetStdErr(dnsID)
	}

	if err := terraform.TerraformInit(); err != nil {
		return err
	}
	if err := terraform.TerraformDestroy(); err != nil {
		return err
	}
	log.Info().Msgf("DNS records for %s from cluster %s were successfully destroyed", d.CurrentDNS.Endpoint, d.ClusterName)

	if err := os.RemoveAll(dnsDir); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", dnsDir, err)
	}

	return nil
}

func (d DNS) generateFiles(dnsID, dnsDir string, dns *pb.DNS, nodeIPs []string) error {
	backend := backend.Backend{
		ProjectName: d.ProjectName,
		ClusterName: dnsID,
		Directory:   dnsDir,
	}

	if err := backend.CreateFiles(); err != nil {
		return err
	}

	providers := provider.Provider{
		ProjectName: d.ProjectName,
		ClusterName: dnsID,
		Directory:   dnsDir,
	}

	if err := providers.CreateProviderDNS(dns); err != nil {
		return err
	}

	if err := utils.CreateKeyFile(dns.Provider.Credentials, dnsDir, dns.Provider.SpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", dns.Provider.SpecName, dnsDir, err)
	}

	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.TerraformerTemplates}
	tpl, err := templateLoader.LoadTemplate(fmt.Sprintf("%s-dns.tpl", dns.Provider.CloudProviderName))
	if err != nil {
		return fmt.Errorf("error while parsing template file dns.tpl for %s : %w", dnsDir, err)
	}

	dnsTemplates := templateUtils.Templates{Directory: dnsDir}
	return dnsTemplates.Generate(tpl, fmt.Sprintf("%s-dns.tf", dns.Provider.CloudProviderName), DNSData{
		DNSZone:      dns.DnsZone,
		HostnameHash: dns.Hostname,
		ClusterName:  d.ClusterName,
		ClusterHash:  d.ClusterHash,
		NodeIPs:      nodeIPs,
		Provider:     dns.Provider,
	})
}

func validateDomain(s string) string {
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

func readDomain(data string) (outputDomain, error) {
	var result outputDomain
	err := json.Unmarshal([]byte(data), &result.Domain)
	return result, err
}
