package templates

import (
	_ "embed"
	"fmt"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
)

//go:embed providers.tpl
var providersTemplate string

type UsedProviders struct {
	ProjectName string
	ClusterName string
	Directory   string
}

// providerTemplateData is data structure passed to providers.tpl
type usedProvidersTemplateData struct {
	Gcp          bool
	Hetzner      bool
	Aws          bool
	Oci          bool
	Azure        bool
	Cloudflare   bool
	HetznerDNS   bool
	GenesisCloud bool
	Openstack    bool
}

// CreateUsedProviderDNS creates provider file used for DNS management.
func (p UsedProviders) CreateUsedProviderDNS(dns *spec.DNS) error {
	template := templateUtils.Templates{Directory: p.Directory}

	tpl, err := templateUtils.LoadTemplate(providersTemplate)
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl for cluster %s: %w", p.ClusterName, err)
	}

	var data usedProvidersTemplateData
	getDNSProvider(dns, &data)

	return template.Generate(tpl, "providers.tf", data)
}

// CreateUsedProvider creates provider file used for infrastructure management.
func (p UsedProviders) CreateUsedProvider(c *spec.ClusterInfoV2) error {
	template := templateUtils.Templates{Directory: p.Directory}

	var data usedProvidersTemplateData
	getProvidersUsed(nodepools.ExtractDynamic(c.NodePools), &data)

	tpl, err := templateUtils.LoadTemplate(providersTemplate)
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl for cluster %s : %w", p.ClusterName, err)
	}

	if err := template.Generate(tpl, "providers.tf", data); err != nil {
		return fmt.Errorf("error while creating provider.tf for %s : %w", p.ClusterName, err)
	}

	return nil
}

// getProvidersUsed modifies providerTemplateData to reflect current providers used.
func getProvidersUsed(nodepools []*spec.DynamicNodePool, data *usedProvidersTemplateData) {
	if len(nodepools) == 0 {
		return
	}

	for _, nodepool := range nodepools {
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
		if nodepool.Provider.CloudProviderName == "genesiscloud" {
			data.GenesisCloud = true
		}
		if nodepool.Provider.CloudProviderName == "openstack" {
			data.Openstack = true
		}
	}
}

// getProvidersUsed modifies providerTemplateData to reflect current providers used in DNS.
func getDNSProvider(dns *spec.DNS, data *usedProvidersTemplateData) {
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
