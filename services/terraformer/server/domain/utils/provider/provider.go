package provider

import (
	"fmt"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
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

func (p Provider) CreateProvider(currentCluster, desiredCluster *pb.ClusterInfo) error {
	template := templateUtils.Templates{Directory: p.Directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.TerraformerTemplates}

	var data templateData
	getProvidersUsed(currentCluster, &data)
	getProvidersUsed(desiredCluster, &data)

	tpl, err := templateLoader.LoadTemplate("providers.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl for cluster %s : %w", p.ClusterName, err)
	}

	if err := template.Generate(tpl, "providers.tf", data); err != nil {
		return fmt.Errorf("error while creating provider.tf for %s : %w", p.ClusterName, err)
	}

	return nil
}

func getProvidersUsed(clusterInfo *pb.ClusterInfo, data *templateData) {
	if clusterInfo == nil {
		return
	}

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
