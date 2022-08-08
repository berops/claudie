package provider

import (
	"fmt"

	"github.com/Berops/platform/utils"
)

type Provider struct {
	ProjectName string
	ClusterName string
	Directory   string
}

func (p Provider) CreateProvider() error {
	template := utils.Templates{Directory: p.Directory}
	templateLoader := utils.TemplateLoader{Directory: utils.TerraformerTemplates}
	tpl, err := templateLoader.LoadTemplate("providers.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file backend.tpl: %v", err)
	}
	err = template.Generate(tpl, "providers.tf", nil)
	if err != nil {
		return fmt.Errorf("error while creating backend files: %v", err)
	}
	return nil
}
