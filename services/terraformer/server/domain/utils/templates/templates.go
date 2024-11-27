package templates

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/go-git/go-git/v5"
)

// ErrEmptyRepository is returned when no repository is to be cloned.
var ErrEmptyRepository = errors.New("no repository to clone")

type Repository struct {
	// TemplatesRootDirectory specifies the root directory where
	// the requested templates repositories will be cloned.
	// Example: TemplatesRootDirectory = "/tmp/"
	//	repository will be cloned to "tmp/
	TemplatesRootDirectory string
}

func DownloadProvider(downloadInto string, provider *spec.Provider) error {
	repo := Repository{
		TemplatesRootDirectory: downloadInto,
	}

	err := repo.Download(provider.GetTemplates())
	if err != nil {
		if errors.Is(err, ErrEmptyRepository) {
			msg := fmt.Sprintf("provider %q does not have a template repository", provider.GetSpecName())
			return fmt.Errorf("%s: %w", msg, err)
		}
		return err
	}
	return nil
}

func (r *Repository) Download(repository *spec.TemplateRepository) error {
	if repository == nil {
		return ErrEmptyRepository
	}

	u, err := url.Parse(repository.Repository)
	if err != nil {
		return fmt.Errorf("%s is not a valid url: %w", repository.Repository, err)
	}

	cloneDirectory := filepath.Join(r.TemplatesRootDirectory, u.Hostname(), u.Path)
	gitDirectory := filepath.Join(cloneDirectory, repository.CommitHash)

	if fileutils.DirectoryExists(gitDirectory) {
		existingMirror, err := git.PlainOpen(gitDirectory)
		if err != nil {
			return fmt.Errorf("%q is not a valid local git repository: %w", gitDirectory, err)
		}

		ref, err := existingMirror.Head()
		if err != nil {
			return fmt.Errorf("failed to read HEAD of local repository %q: %w", gitDirectory, err)
		}

		if ref.Hash().String() == repository.CommitHash {
			logs := new(bytes.Buffer)
			sparseCheckout := exec.Command("git", "sparse-checkout", "set", strings.Trim(repository.Path, "/"))
			sparseCheckout.Dir = gitDirectory
			sparseCheckout.Stdout = logs
			sparseCheckout.Stderr = logs

			if err := sparseCheckout.Run(); err != nil {
				return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", repository.Repository, err, logs.String())
			}

			logs.Reset()
			args := []string{"checkout"}
			if repository.Tag != nil {
				args = append(args, *repository.Tag)
			}
			checkout := exec.Command("git", args...)
			checkout.Dir = gitDirectory
			checkout.Stdout = logs
			checkout.Stderr = logs

			if err := checkout.Run(); err != nil {
				return fmt.Errorf("failed to checkout for %q, repository %q: %w: %s", args, repository.Repository, err, logs.String())
			}

			return nil
		}

		// on mismatch re-download the repo.
		if err := os.RemoveAll(gitDirectory); err != nil {
			return fmt.Errorf("failed to delete local clone %q: %w", cloneDirectory, err)
		}
		// fallthrough, continue with the cloning below
	}

	if err := fileutils.CreateDirectory(cloneDirectory); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", cloneDirectory, err)
	}

	logs := new(bytes.Buffer)
	clone := exec.Command("git", "clone", "--no-checkout", repository.Repository, repository.CommitHash)
	clone.Dir = cloneDirectory
	clone.Stdout = logs
	clone.Stderr = logs

	if err := clone.Run(); err != nil {
		return fmt.Errorf("failed to clone %q: %w: %s", repository.Repository, err, logs.String())
	}

	logs.Reset()
	sparseCheckout := exec.Command("git", "sparse-checkout", "set", strings.Trim(repository.Path, "/"))
	sparseCheckout.Dir = gitDirectory
	sparseCheckout.Stdout = logs
	sparseCheckout.Stderr = logs

	if err := sparseCheckout.Run(); err != nil {
		return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", repository.Repository, err, logs.String())
	}

	logs.Reset()

	checkout := exec.Command("git", "checkout", repository.CommitHash)
	checkout.Dir = gitDirectory
	checkout.Stdout = logs
	checkout.Stderr = logs

	if err := checkout.Run(); err != nil {
		return fmt.Errorf("failed to checkout for %q, repository %q: %w: %s", repository.CommitHash, repository.Repository, err, logs.String())
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
