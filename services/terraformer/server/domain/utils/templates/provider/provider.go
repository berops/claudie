package provider

import (
	_ "embed"
	"fmt"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
)

//go:embed providers.tpl
var providersTemplate string

// templateData is data structure passed to providers.tpl
type templateData struct {
	Gcp          bool
	Hetzner      bool
	Aws          bool
	Oci          bool
	Azure        bool
	Cloudflare   bool
	HetznerDNS   bool
	GenesisCloud bool
}

func CreateDNS(targetDir string, dns *pb.DNS) error {
	template := templateUtils.Templates{
		Directory: targetDir,
	}

	tpl, err := templateUtils.LoadTemplate(providersTemplate)
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl: %w", err)
	}

	var data templateData
	getDNSProvider(dns, &data)
	return template.Generate(tpl, "providers.tf", data)
}

func CreateNodepool(targetDir string, np *pb.NodePool) error {
	template := templateUtils.Templates{
		Directory: targetDir,
	}

	var data templateData
	getProvidersUsed(np.GetDynamicNodePool(), &data)

	tpl, err := templateUtils.LoadTemplate(providersTemplate)
	if err != nil {
		return fmt.Errorf("error while parsing template file providers.tpl:%w", err)
	}

	if err := template.Generate(tpl, "providers.tf", data); err != nil {
		return fmt.Errorf("error while creating provider.tf: %w", err)
	}

	return nil
}

// getProvidersUsed modifies templateData to reflect current providers used.
func getProvidersUsed(np *pb.DynamicNodePool, data *templateData) {
	if np == nil {
		return
	}
	switch np.Provider.CloudProviderName {
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
	case "genesiscloud":
		data.GenesisCloud = true
	}
}

// getProvidersUsed modifies templateData to reflect current providers used in DNS.
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
