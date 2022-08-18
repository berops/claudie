package provider

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
)

// Provider package struct
type Provider struct {
	ProjectName string
	ClusterName string
	Directory   string
}

// Data structure passed to providers.tpl
type templateData struct {
	Gcp     bool
	Hetzner bool
}

func (p Provider) CreateProvider(clusterInfo *pb.ClusterInfo) error {
	template := utils.Templates{Directory: p.Directory}
	templateLoader := utils.TemplateLoader{Directory: utils.TerraformerTemplates}
	data := getProvidersUsed(clusterInfo)
	tpl, err := templateLoader.LoadTemplate("providers.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file backend.tpl: %v", err)
	}
	err = template.Generate(tpl, "providers.tf", data)
	if err != nil {
		return fmt.Errorf("error while creating backend files: %v", err)
	}
	return nil
}

func getProvidersUsed(clusterInfo *pb.ClusterInfo) templateData {
	var data = templateData{Gcp: false, Hetzner: false}
	for _, nodepool := range clusterInfo.NodePools {
		if nodepool.Provider.CloudProviderName == "gcp" {
			data.Gcp = true
		}
		if nodepool.Provider.CloudProviderName == "hetzner" {
			data.Hetzner = true
		}
	}
	return data
}
