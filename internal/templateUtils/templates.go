package templateUtils

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// directory - output directory
// MUST be relative to base directory, i.e. services/terraformer/etc
type Templates struct {
	Directory string
}

// creates a  file from template and saves it to the directory specified in Templates
// the directory MUST be relative to base directory, i.e. services/terraformer/templates
func (t Templates) Generate(tpl *template.Template, outputFile string, d interface{}) error {
	generatedFile := filepath.Join(t.Directory, outputFile)
	// make sure the t.Directory exists, if not, create it
	if _, err := os.Stat(t.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(t.Directory, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s : %w", t.Directory, err)
		}
	}
	f, err := os.Create(generatedFile)
	if err != nil {
		return fmt.Errorf("failed to create the %s file in %s directory : %w", generatedFile, t.Directory, err)
	}
	if err := tpl.Execute(f, d); err != nil {
		return fmt.Errorf("failed to execute the template file for %s : %w", generatedFile, err)
	}
	return nil
}

// creates a  file from template and returns it as a string variable
// returns error if not successful, generated template as a string and nil otherwise
func (t Templates) GenerateToString(tpl *template.Template, d interface{}) (string, error) {
	var buff bytes.Buffer
	err := tpl.Execute(&buff, d)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

// LoadTemplate creates template instance with auxiliary functions from specified template.
func LoadTemplate(tplFile string) (*template.Template, error) {
	tpl, err := template.New("").Funcs(template.FuncMap{
		"replaceAll":             strings.ReplaceAll,
		"trimPrefix":             strings.TrimPrefix,
		"extractNetmaskFromCIDR": ExtractNetmaskFromCIDR,
	}).Parse(tplFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the template file : %w", err)
	}
	return tpl, nil
}

// ExtractNetmaskFromCIDR extracts the netmask from the CIDR notation.
func ExtractNetmaskFromCIDR(cidr string) string {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}

	ones, _ := n.Mask.Size()
	return fmt.Sprintf("%v", ones)
}
