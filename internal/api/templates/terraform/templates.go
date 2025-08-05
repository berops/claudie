package terraform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/api/templates"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
)

// TODO: what we need to do is create a generic git download and sparse checkout that would then be used
// for any templates, not just terraform.

// ErrEmptyRepository is returned when no repository is to be cloned.
var ErrEmptyRepository = errors.New("no repository to clone")

type Repository struct {
	// TemplatesRootDirectory specifies the root directory where
	// the requested templates repositories will be cloned.
	// Example:
	//  TemplatesRootDirectory = "/tmp/", repository will be cloned to "tmp/"
	TemplatesRootDirectory string
}

func DownloadProvider(downloadInto string, provider *spec.Provider) error {
	// Provider cannot have empty templates, if not supplied by the user, default ones are used.
	t := provider.GetTemplates()
	repository := templates.Repository{
		Repository: t.Repository,
		Path:       t.Path,
		Commit:     *t.Tag, // TODO: change me once done. This must always exists and never be a pointer... Also adjust tests ???
		CommitHash: t.CommitHash,
	}
	if err := templates.Download(downloadInto, repository); err != nil {
		return fmt.Errorf("provider %q templates error: %w", provider.GetSpecName(), err)
	}
	return nil
}

type Generator struct {
	// ID is the ClusterID or DnsID.
	ID string
	// Where the templates should be generated.
	TargetDirectory string
	// Root directory where the template files were downloaded
	// To this directory the relative path of the templates will
	// be added to read the templates for each nodepool.
	ReadFromDirectory string
	// TemplatePath is the path from the Root directory of the templates
	// to the requested provider templates.
	TemplatePath string
	// Fingerprint is a sequence of bytes that uniquely identifies the
	// templates attached to a provider. For example if two providers use
	// the same templates the fingerprint must be unique so that no collisions
	// occur when generating the template files.
	Fingerprint string
}

func (g *Generator) GenerateProvider(data *Provider) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "provider"),
		data.Provider.SpecName,
		data,
	)
}

func (g *Generator) GenerateNetworking(data *Networking) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "networking"),
		data.Provider.SpecName,
		data,
	)
}

func (g *Generator) GenerateNodes(data *Nodepools) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "nodepool"),
		data.NodePools[0].Details.GetProvider().GetSpecName(),
		data,
	)
}

func (g *Generator) GenerateDNS(data *DNS) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "dns"),
		data.Provider.SpecName,
		data,
	)
}

func (g *Generator) generateTemplates(dir, specName string, data any) error {
	type fingerPrintedData struct {
		// Data is data passed to the template generator (one of the above).
		Data any
		// Fingerprint is the checksum of the templates of a given nodepool.
		Fingerprint string
	}

	var (
		targetDirectory = templateUtils.Templates{Directory: g.TargetDirectory}
	)

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", dir, err)
	}

	for _, gotpl := range files {
		if gotpl.IsDir() {
			continue
		}

		if !strings.HasSuffix(gotpl.Name(), ".tpl") {
			continue
		}

		file, err := os.ReadFile(filepath.Join(dir, gotpl.Name()))
		if err != nil {
			return fmt.Errorf("error while reading template file %s in %s: %w", gotpl, dir, err)
		}

		tpl, err := templateUtils.LoadTemplate(string(file))
		if err != nil {
			return fmt.Errorf("error while parsing template file %s from %s : %w", gotpl.Name(), dir, err)
		}

		gotpl := strings.TrimSuffix(gotpl.Name(), ".tpl")
		outputFile := fmt.Sprintf("%s-%s-%s-%s.tf", g.ID, specName, gotpl, g.Fingerprint)

		data := fingerPrintedData{
			Data:        data,
			Fingerprint: g.Fingerprint,
		}

		if err := targetDirectory.Generate(tpl, outputFile, data); err != nil {
			return fmt.Errorf("error while generating %s file : %w", outputFile, err)
		}
	}

	return nil
}
