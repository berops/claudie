package loadbalancer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/backend"
	"github.com/Berops/platform/services/terraformer/server/clusterBuilder"
	"github.com/Berops/platform/services/terraformer/server/templates"
	"github.com/Berops/platform/services/terraformer/server/terraform"
	"github.com/rs/zerolog/log"
)

type DNS struct {
	ClusterName     string
	ClusterHash     string
	DNSZone         string
	NodeIPs         []string
	Project         string
	CurrentProvider *pb.Provider
	DesiredProvider *pb.Provider
	ProjectName     string
	Hostname        string
}

type DNSData struct {
	ClusterName  string
	ClusterHash  string
	HostnameHash string
	DNSZone      string
	NodeIPs      []string
	Project      string
	Provider     *pb.Provider
}

type outputDomain struct {
	Domain map[string]string `json:"-"`
}

func (d DNS) CreateDNSrecords() (string, error) {
	clusterID := fmt.Sprintf("%s-%s", d.ClusterName, d.ClusterHash)
	dnsID := fmt.Sprintf("%s-dns", clusterID)
	dnsDir := filepath.Join(clusterBuilder.Output, dnsID)
	terraform := terraform.Terraform{Directory: dnsDir}
	//check if DNS provider is unchanged
	changedDNS, err := d.checkDNSProvider()
	if err != nil {
		return "", fmt.Errorf("error while checking the DNS provider credentials: %v", err)
	}
	if changedDNS {
		//destroy old DNS records
		err := d.generateFiles(dnsID, dnsDir, d.CurrentProvider)
		if err != nil {
			return "", fmt.Errorf("error while creating dns records for %s : %v", dnsID, err)
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
	err = d.generateFiles(dnsID, dnsDir, d.DesiredProvider)
	if err != nil {
		return "", fmt.Errorf("error while creating dns records for %s : %v", dnsID, err)
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
	err := d.generateFiles(dnsID, dnsDir, d.CurrentProvider)
	if err != nil {
		return fmt.Errorf("error while creating dns records for %s : %v", dnsID, err)
	}
	// create dns records with terraform
	terraform := terraform.Terraform{Directory: dnsDir}
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

func (d DNS) generateFiles(dnsID, dnsDir string, provider *pb.Provider) error {
	// generate backend
	backend := backend.Backend{ProjectName: d.ProjectName, ClusterName: dnsID, Directory: dnsDir}
	err := backend.CreateFiles()
	if err != nil {
		return err
	}

	DNSTemplates := templates.Templates{Directory: dnsDir}
	dnsData := d.getDNSData(provider)
	return DNSTemplates.Generate("dns.tpl", "dns.tf", dnsData)
}

func (d DNS) checkDNSProvider() (bool, error) {
	// DNS not yet created
	if d.CurrentProvider == nil {
		return false, nil
	}
	// DNS provider are same
	if d.CurrentProvider.Name == d.DesiredProvider.Name {
		if d.CurrentProvider.Credentials == d.DesiredProvider.Credentials {
			return false, nil
		}
	}
	return true, nil
}

// function returns pair of strings, first the hash hostname, second the zone
func (d DNS) getDNSData(provider *pb.Provider) DNSData {
	DNSData := DNSData{
		DNSZone:      d.DNSZone,
		HostnameHash: d.Hostname,
		ClusterName:  d.ClusterName,
		ClusterHash:  d.ClusterHash,
		NodeIPs:      d.NodeIPs,
		Project:      d.Project,
		Provider:     provider,
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
