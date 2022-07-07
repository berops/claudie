package backend

import (
	"fmt"

	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
)

var (
	minioURL = urls.MinioURL
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

type templateData struct {
	ProjectName string
	ClusterName string
	MinioURL    string
}

// function CreateFiles will create a backend.tf file from template
func (b Backend) CreateFiles() error {
	template := utils.Templates{Directory: b.Directory}
	templateLoader := utils.TemplateLoader{Directory: utils.TerraformerTemplates}
	tpl, err := templateLoader.LoadTemplate("backend.tpl")
	if err != nil {
		return fmt.Errorf("error while parsing template file backend.tpl: %v", err)
	}
	data := templateData{ProjectName: b.ProjectName, ClusterName: b.ClusterName, MinioURL: minioURL}
	err = template.Generate(tpl, "backend.tf", data)
	if err != nil {
		return fmt.Errorf("error while creating backend files: %v", err)
	}
	return nil
}
