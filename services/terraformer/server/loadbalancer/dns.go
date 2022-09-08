package loadbalancer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	comm "github.com/Berops/claudie/internal/command"

	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/terraformer/server/backend"
	"github.com/Berops/claudie/services/terraformer/server/clusterBuilder"
	"github.com/Berops/claudie/services/terraformer/server/terraform"
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

func (d DNS) CreateDNSrecords() (string, error) {
	clusterID := fmt.Sprintf("%s-%s", d.ClusterName, d.ClusterHash)
	dnsID := fmt.Sprintf("%s-dns", clusterID)
	dnsDir := filepath.Join(clusterBuilder.Output, dnsID)
	terraform := terraform.Terraform{Directory: dnsDir, StdOut: comm.GetStdOut(dnsID), StdErr: comm.GetStdErr(dnsID)}
	//check if DNS provider is unchanged
	if utils.ChangedDNSProvider(d.CurrentDNS, d.DesiredDNS) {
		log.Info().Msgf("Destroying old DNS records")
		//destroy old DNS records
		err := d.generateFiles(dnsID, dnsDir, d.CurrentDNS, d.CurrentNodeIPs)
		if err != nil {
			return "", fmt.Errorf("error while creating dns .tf files for %s : %v", dnsID, err)
		}
		// destroy old dns records with terraform
		err = terraform.TerraformInit()
		if err != nil {
			return "", err
		}
		err = terraform.TerraformDestroy()
		if err != nil {
			return "", err
		}
	}
	// create files
	err := d.generateFiles(dnsID, dnsDir, d.DesiredDNS, d.DesiredNodeIPs)
	if err != nil {
		return "", fmt.Errorf("error while creating dns .tf files for %s : %v", dnsID, err)
	}
	// create dns records with terraform
	err = terraform.TerraformInit()
	if err != nil {
		return "", err
	}
	err = terraform.TerraformApply()
	if err != nil {
		return "", err
	}

	// read output and return it
	outputID := fmt.Sprintf("%s-%s", clusterID, "endpoint")
	output, err := terraform.TerraformOutput(clusterID)
	if err != nil {
		log.Error().Msgf("Error while getting output from terraform: %v", err)
		return "", err
	}
	out, err := readDomain(output)
	if err != nil {
		log.Error().Msgf("Error while reading the terraform output: %v", err)
		return "", err
	}
	// Clean after terraform
	if err := os.RemoveAll(dnsDir); err != nil {
		return validateDomain(out.Domain[outputID]), fmt.Errorf("error while deleting files: %v", err)
	}
	return validateDomain(out.Domain[outputID]), nil
}

func (d DNS) DestroyDNSrecords() error {
	dnsID := fmt.Sprintf("%s-%s-dns", d.ClusterName, d.ClusterHash)
	dnsDir := filepath.Join(clusterBuilder.Output, dnsID)
	// create files
	err := d.generateFiles(dnsID, dnsDir, d.CurrentDNS, d.CurrentNodeIPs)
	if err != nil {
		return fmt.Errorf("error while creating dns records for %s : %v", dnsID, err)
	}
	// create dns records with terraform
	terraform := terraform.Terraform{Directory: dnsDir, StdOut: comm.GetStdOut(dnsID), StdErr: comm.GetStdErr(dnsID)}
	err = terraform.TerraformInit()
	if err != nil {
		return err
	}
	err = terraform.TerraformDestroy()
	if err != nil {
		return err
	}

	// Clean after terraform
	if err := os.RemoveAll(dnsDir); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}

func (d DNS) generateFiles(dnsID, dnsDir string, dns *pb.DNS, nodeIPs []string) error {
	// generate backend
	backend := backend.Backend{ProjectName: d.ProjectName, ClusterName: dnsID, Directory: dnsDir}
	err := backend.CreateFiles()
	if err != nil {
		return err
	}

	// save provider cred file
	if err = utils.CreateKeyFile(dns.Provider.Credentials, dnsDir, dns.Provider.SpecName); err != nil {
		log.Error().Msgf("Error creating provider credential key file: %v", err)
		return err
	}

	DNSTemplates := templateUtils.Templates{Directory: dnsDir}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.TerraformerTemplates}
	tpl, err := templateLoader.LoadTemplate("dns.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file backend.tpl: %v", err)
	}
	dnsData := d.getDNSData(dns, nodeIPs)
	return DNSTemplates.Generate(tpl, "dns.tf", dnsData)
}

// function returns pair of strings, first the hash hostname, second the zone
func (d DNS) getDNSData(dns *pb.DNS, nodeIPs []string) DNSData {
	DNSData := DNSData{
		DNSZone:      dns.DnsZone,
		HostnameHash: dns.Hostname,
		ClusterName:  d.ClusterName,
		ClusterHash:  d.ClusterHash,
		NodeIPs:      nodeIPs,
		Provider:     dns.Provider,
	}
	return DNSData
}

func validateDomain(s string) string {
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

func readDomain(data string) (outputDomain, error) {
	var result outputDomain
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.Domain)
	return result, err
}
