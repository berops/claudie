package extofu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/tmplutils"
)

type Generator struct {
	// ID is the ClusterID or DnsID.
	ID string

	// Where the templates should be generated to.
	TargetDirectory string

	// Root directory where the template files were downloaded.
	// To this directory the relative path of the templates will
	// be added to read the templates for each nodepool.
	ReadFromDirectory string

	// TemplatePath is the path from the Root directory of the templates
	// to the requested provider templates.
	TemplatePath string

	// Fingerprint is a sequence of bytes that uniquely identifies the
	// templates attached to a provider. For example if two providers use
	// the same templates, the fingerprint must be unique so that no collisions
	// occur when generating the template files.
	Fingerprint string
}

func (g *Generator) GenerateNetworking(data *Networking) error {
	_, err := g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "networking"),
		data.Provider.SpecName,
		data,
		func(s string) bool { return s != ProviderFile },
	)
	return err
}

func (g *Generator) GenerateNetworkingProvider(data *Networking) error {
	rendered, err := g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "networking"),
		data.Provider.SpecName,
		data,
		func(s string) bool { return s == ProviderFile },
	)
	if err != nil {
		return err
	}
	if rendered == 0 {
		return fmt.Errorf("no %q inside 'networking' directory", ProviderFile)
	}
	return nil
}

func (g *Generator) GenerateNodes(data *Nodepool) error {
	_, err := g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "nodepool"),
		data.NodePool.Details.GetProvider().GetSpecName(),
		data,
	)
	return err
}

func (g *Generator) GenerateDNS(data *DNS) error {
	_, err := g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "dns"),
		data.Provider.SpecName,
		data,
	)
	return err
}

// generateTemplates generates all of the files with the '.tpl' suffix in the specified directory.
//
// To filter out only specific files, the optional filters slice can be specified for file names.
func (g *Generator) generateTemplates(dir, specName string, data any, filters ...func(string) bool) (int, error) {
	type fingerPrintedData struct {
		// Data is data passed to the template generator (one of the above).
		Data any
		// Fingerprint is the checksum of the templates of a given nodepool.
		Fingerprint string
	}

	targetDirectory := tmplutils.Templates{
		Directory: g.TargetDirectory,
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory %q: %w", dir, err)
	}

	if err := fileutils.CreateDirectory(targetDirectory.Directory); err != nil {
		return 0, err
	}

	rendered := 0

outer:
	for _, gotpl := range files {
		if gotpl.IsDir() {
			continue
		}

		for _, f := range filters {
			if !f(strings.ToLower(gotpl.Name())) {
				continue outer
			}
		}

		if !strings.HasSuffix(gotpl.Name(), ".tpl") {
			continue
		}

		file, err := os.ReadFile(filepath.Join(dir, gotpl.Name()))
		if err != nil {
			return rendered, fmt.Errorf("error while reading template file %s in %s: %w", gotpl.Name(), dir, err)
		}

		tpl, err := tmplutils.LoadTemplate(string(file))
		if err != nil {
			return rendered, fmt.Errorf("error while parsing template file %s from %s : %w", gotpl.Name(), dir, err)
		}

		gotpl := strings.TrimSuffix(gotpl.Name(), ".tpl")
		outputFile := fmt.Sprintf("%s-%s-%s-%s.tf", g.ID, specName, gotpl, g.Fingerprint)

		data := fingerPrintedData{
			Data:        data,
			Fingerprint: g.Fingerprint,
		}

		if err := targetDirectory.Generate(tpl, outputFile, data); err != nil {
			return rendered, fmt.Errorf("error while generating %s file : %w", outputFile, err)
		}

		rendered += 1
	}

	return rendered, nil
}
