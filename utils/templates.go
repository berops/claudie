package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"text/template"

	"github.com/rs/zerolog/log"
)

const (
	baseDirectory         = "." //NOTE: left it here since it might be changed later
	TerraformerTemplates  = "services/terraformer/templates"
	WireguardianTemplates = "services/wireguardian/server"
	KuberTemplates        = "services/kuber/templates"
	KubeElevenTemplates   = "services/kube-eleven/server/templates"
)

// directory - output directory
// MUST be relative to base directory, i.e. services/terraformer/etc
type Templates struct {
	Directory string
}

// directory - output directory
// MUST be relative to base directory, i.e. services/terraformer/etc
type TemplateLoader struct {
	Directory string
}

// creates a  file from template and saves it to the directory specified in Templates
// the directory MUST be relative to base directory, i.e. services/terraformer/templates
func (t Templates) Generate(tpl *template.Template, outputFile string, d interface{}) error {
	//append the relative path in order to have a path from base directory
	t.Directory = filepath.Join(baseDirectory, t.Directory)
	generatedFile := filepath.Join(t.Directory, outputFile)
	// make sure the t.Directory exists, if not, create it
	if _, err := os.Stat(t.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(t.Directory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}
	log.Info().Msgf("Creating %s \n", generatedFile)
	f, err := os.Create(generatedFile)
	if err != nil {
		return fmt.Errorf("failed to create the %s file: %v", t.Directory, err)
	}
	if err := tpl.Execute(f, d); err != nil {
		return fmt.Errorf("failed to execute the template file: %v", err)
	}
	return nil
}

//loads the template from directory specified in TemplateLoader
// the directory MUST be relative to base directory, i.e. services/terraformer/templates
func (tl TemplateLoader) LoadTemplate(tplFile string) (*template.Template, error) {
	tpl, err := template.ParseFiles(filepath.Join(baseDirectory, tl.Directory, tplFile))
	if err != nil {
		return nil, fmt.Errorf("failed to load the template file: %v", err)
	}
	return tpl, nil
}
