package provider

import (
	"fmt"

	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/proto/pb"
)

// Provider package struct
type Provider struct {
	ProjectName string
	ClusterName string
	Directory   string
}

// Data structure passed to providers.tpl
type templateData struct {
	Gcp        bool
	Hetzner    bool
	Aws        bool
	Oci        bool
	Azure      bool
	Cloudflare bool
	HetznerDNS bool
}

func (p Provider) CreateProviderDNS(dns *pb.DNS) error {
	template := templateUtils.Templates{Directory: p.Directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.TerraformerTemplates}

	tpl, err := templateLoader.LoadTemplate("providers.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl for cluster %s: %w", p.ClusterName, err)
	}

	var data templateData
	getDNSProvider(dns, &data)
	return template.Generate(tpl, "providers.tf", data)
}

func (p Provider) CreateProvider(clusterInfo *pb.ClusterInfo) error {
	template := templateUtils.Templates{Directory: p.Directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.TerraformerTemplates}
	data := getProvidersUsed(clusterInfo)
	tpl, err := templateLoader.LoadTemplate("providers.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl for cluster %s : %w", p.ClusterName, err)
	}
	err = template.Generate(tpl, "providers.tf", data)
	if err != nil {
		return fmt.Errorf("error while creating provider.tf for %s : %w", p.ClusterName, err)
	}
	return nil
}

func getProvidersUsed(clusterInfo *pb.ClusterInfo) templateData {
	var data = templateData{Gcp: false, Hetzner: false, Oci: false, Aws: false, Azure: false}
	for _, nodepool := range clusterInfo.NodePools {
		if nodepool.Provider.CloudProviderName == "gcp" {
			data.Gcp = true
		}
		if nodepool.Provider.CloudProviderName == "hetzner" {
			data.Hetzner = true
		}
		if nodepool.Provider.CloudProviderName == "aws" {
			data.Aws = true
		}
		if nodepool.Provider.CloudProviderName == "oci" {
			data.Oci = true
		}
		if nodepool.Provider.CloudProviderName == "azure" {
			data.Azure = true
		}
	}
	return data
}

func getDNSProvider(dns *pb.DNS, data *templateData) {
	if dns == nil {
		return
	}

	switch dns.Provider.CloudProviderName {
	case "gcp":
		data.Gcp = true
	case "hetzner":
		data.Hetzner = true
	case "aws":
		data.Aws = true
	case "oci":
		data.Oci = true
	case "azure":
		data.Azure = true
	case "cloudflare":
		data.Cloudflare = true
	case "hetznerdns":
		data.HetznerDNS = true
	}
}
