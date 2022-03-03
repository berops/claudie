package templates

import (
	"fmt"
	"os"
	"path/filepath"

	"text/template"

	"github.com/rs/zerolog/log"
)

const (
	templatePath = "services/terraformer/templates"
)

// directory - output directory
type Templates struct {
	Directory string
}

func (t Templates) Generate(tplFile, outputFile string, d interface{}) error {
	generatedFile := filepath.Join(t.Directory, outputFile)
	// make sure the t.Directory exists, if not, create it
	if _, err := os.Stat(t.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(t.Directory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}

	tpl, err := template.ParseFiles(filepath.Join(templatePath, tplFile))
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
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
