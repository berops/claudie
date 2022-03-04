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
	ClusterName string
	ClusterHash string
	DNSZone     string
	NodeIPs     []string
	Project     string
	Provider    *pb.Provider
	ProjectName string
	Hostname    string
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
	// create files
	err := d.generateFiles(dnsID, dnsDir)
	if err != nil {
		return "", fmt.Errorf("error while creating dns records for %s : %v", dnsID, err)
	}
	// create dns records with terraform
	terraform := terraform.Terraform{Directory: dnsDir}
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
	err := d.generateFiles(dnsID, dnsDir)
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

func (d DNS) generateFiles(dnsID, dnsDir string) error {
	// generate backend
	backend := backend.Backend{ProjectName: d.ProjectName, ClusterName: dnsID, Directory: dnsDir}
	err := backend.CreateFiles()
	if err != nil {
		return err
	}

	DNSTemplates := templates.Templates{Directory: dnsDir}
	dnsData := d.getDNSData()
	return DNSTemplates.Generate("dns.tpl", "dns.tf", dnsData)
}

// function returns pair of strings, first the hash hostname, second the zone
func (d DNS) getDNSData() DNSData {
	DNSData := DNSData{
		DNSZone:      d.DNSZone,
		HostnameHash: d.Hostname,
		ClusterName:  d.ClusterName,
		ClusterHash:  d.ClusterHash,
		NodeIPs:      d.NodeIPs,
		Project:      d.Project,
		Provider:     d.Provider,
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
