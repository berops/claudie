package backend

import (
	"fmt"

	"github.com/Berops/platform/utils"
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

func (b Backend) CreateFiles() error {
	template := utils.Templates{Directory: b.Directory}
	templateLoader := utils.TemplateLoader{Directory: utils.TerraformerTemplates}
	tpl, err := templateLoader.LoadTemplate("backend.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file backend.tpl: %v", err)
	}
	err = template.Generate(tpl, "backend.tf", b)
	if err != nil {
		return fmt.Errorf("error while creating backend files: %v", err)
	}
	return nil
}
