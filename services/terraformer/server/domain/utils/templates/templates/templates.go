package templates

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

// EmptyRepositoryErr is returned when no repository is to be cloned.
var EmptyRepositoryErr = errors.New("no repository to clone")

type Repository struct {
	// TemplatesRootDirectory specifies the root directory where
	// the requested templates repositories will be cloned.
	// Example: TemplatesRootDirectory = "/tmp/"
	//	repository will be cloned to "tmp/
	TemplatesRootDirectory string
}

func DownloadProvider(downloadInto string, provider *pb.Provider) error {
	repo := Repository{
		TemplatesRootDirectory: downloadInto,
	}

	err := repo.Download(provider.GetTemplates())
	if err != nil {
		if errors.Is(err, EmptyRepositoryErr) {
			msg := fmt.Sprintf("provider %q does not have a template repository", provider.GetSpecName())
			return fmt.Errorf("%s: %w", msg, err)
		}
		return err
	}
	return nil
}

func (r *Repository) Download(repository *pb.TemplateRepository) error {
	if repository == nil {
		return EmptyRepositoryErr
	}

	u, err := url.Parse(repository.Repository)
	if err != nil {
		return fmt.Errorf("%s is not a valid url: %w", repository.Repository, err)
	}

	repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{URL: repository.Repository})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %q: %w", repository.Repository, err)
	}

	var targetCommit *plumbing.Reference
	if repository.Tag != nil {
		// TODO: go-git client doesn't correctly handle sparse-checkout
		// ref: https://github.com/go-git/go-git/issues/90
		// once implemented replace shell-out code for direct dependency via go-git.
		if targetCommit, err = repo.Tag(*repository.Tag); err != nil {
			return fmt.Errorf("repository %q does not have tag %q: %w", repository.Repository, *repository.Tag, err)
		}
	} else {
		if targetCommit, err = repo.Head(); err != nil {
			return fmt.Errorf("failed to read HEAD of repository %q: %w", repository.Repository, err)
		}
	}

	// If no tag is specified always use the latest commit from the HEAD of the master branch.
	tagName := "latest"
	if repository.Tag != nil {
		tagName = *repository.Tag
	}

	cloneDirectory := filepath.Join(r.TemplatesRootDirectory, u.Hostname(), u.Path)
	gitDirectory := filepath.Join(cloneDirectory, tagName)

	if utils.DirectoryExists(gitDirectory) {
		existingMirror, err := git.PlainOpen(gitDirectory)
		if err != nil {
			return fmt.Errorf("%q is not a valid local git repository: %w", gitDirectory, err)
		}

		ref, err := existingMirror.Head()
		if err != nil {
			return fmt.Errorf("failed to read HEAD of local repository %q: %w", gitDirectory, err)
		}

		if ref.Hash() == targetCommit.Hash() {
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

	if err := utils.CreateDirectory(cloneDirectory); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", cloneDirectory, err)
	}

	logs := new(bytes.Buffer)
	clone := exec.Command("git", "clone", "--no-checkout", repository.Repository, tagName)
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
}

func (g *Generator) GenerateProvider(data *ProviderData) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "provider"),
		data.Provider.SpecName,
		data,
	)
}

func (g *Generator) GenerateNetworking(data *NetworkingData) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "networking"),
		data.Provider.SpecName,
		data,
	)
}

func (g *Generator) GenerateNodes(data *NodepoolsData) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "nodepool"),
		data.NodePools[0].Details.GetProvider().GetSpecName(),
		data,
	)
}

func (g *Generator) GenerateDNS(data *DNSData) error {
	return g.generateTemplates(
		filepath.Join(g.ReadFromDirectory, g.TemplatePath, "dns"),
		data.Provider.SpecName,
		data,
	)
}

func mustParseURL(s *url.URL, err error) *url.URL {
	if err != nil {
		panic(err)
	}
	return s
}

func ExtractTargetPath(repository *pb.TemplateRepository) string {
	tagName := "latest"
	if repository.Tag != nil {
		tagName = *repository.Tag
	}
	u := mustParseURL(url.Parse(repository.Repository))
	return filepath.Join(
		u.Hostname(),
		u.Path,
		tagName,
		repository.Path,
	)
}

func Fingerprint(s string) string {
	digest := sha512.Sum512_256([]byte(s))
	return hex.EncodeToString(digest[:16])
}

func (g *Generator) generateTemplates(dir, specName string, data any) error {
	var (
		fp              = Fingerprint(g.TemplatePath)
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
			return fmt.Errorf("error while parsing template file %s from %s : %w", gotpl, dir, err)
		}

		outputFile := fmt.Sprintf("%s-%s-%s-%s.tf", g.ID, specName, gotpl, fp)

		data := fingerPrintedData{
			Data:        data,
			Fingerprint: fp,
		}

		if err := targetDirectory.Generate(tpl, outputFile, data); err != nil {
			return fmt.Errorf("error while generating %s file : %w", outputFile, err)
		}
	}

	return nil
}
