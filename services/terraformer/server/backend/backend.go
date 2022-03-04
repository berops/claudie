package backend

import (
	"fmt"

	"github.com/Berops/platform/services/terraformer/server/templates"
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

func (b Backend) CreateFiles() error {
	template := templates.Templates{Directory: b.Directory}
	err := template.Generate("backend.tpl", "backend.tf", b)
	if err != nil {
		return fmt.Errorf("error while creating backend files: %v", err)
	}
	return nil
}
