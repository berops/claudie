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

func DownloadForNodepools(downloadInto string, nodepools []*pb.NodePool) error {
	for _, np := range nodepools {
		if np.GetDynamicNodePool() == nil {
			continue
		}

		repo := Repository{
			TemplatesRootDirectory: downloadInto,
		}

		err := repo.Download(np.GetDynamicNodePool().GetTemplates())
		if err != nil {
			if errors.Is(err, EmptyRepositoryErr) {
				msg := fmt.Sprintf("nodepool %q does not have a template repository", np.Name)
				return fmt.Errorf("%s: %w", msg, err)
			}
			return err
		}
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

	cloneDirectory := filepath.Join(r.TemplatesRootDirectory, u.Hostname(), u.Path)
	gitDirectory := filepath.Join(cloneDirectory, repository.Tag)

	if utils.DirectoryExists(gitDirectory) {
		existingMirror, err := git.PlainOpen(gitDirectory)
		if err != nil {
			return fmt.Errorf("%q is not a valid local git repository: %w", gitDirectory, err)
		}

		tag, err := existingMirror.Tag(repository.Tag)
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return fmt.Errorf("existing repository %q does not have tag %q: %w", repository.Repository, repository.Tag, err)
		}
		if err != nil {
			return fmt.Errorf("failed to resolve tag %q for local repository %q: %w", repository.Tag, gitDirectory, err)
		}

		ref, err := existingMirror.Head()
		if err != nil {
			return fmt.Errorf("failed to read HEAD of local repository %q: %w", gitDirectory, err)
		}

		if ref.Hash() == tag.Hash() {
			return nil
		}

		// on mismatch re-download the repo.
		if err := os.RemoveAll(gitDirectory); err != nil {
			return fmt.Errorf("failed to delete local clone %q: %w", cloneDirectory, err)
		}
		// fallthrough, continue with the cloning below
	}

	// Check if requested tag exists for repository.
	repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{URL: repository.Repository})

	if err != nil {
		return fmt.Errorf("failed to clone repository: %q: %w", repository.Repository, err)
	}

	// TODO: go-git client doesn't corretly handle sparse-checkout
	// ref: https://github.com/go-git/go-git/issues/90
	// once implemented replace shell-out code for direct dependency via go-git.
	if _, err := repo.Tag(repository.Tag); err != nil {
		return fmt.Errorf("repository %q does not have tag %q: %w", repository.Repository, repository.Tag, err)
	}

	if err := utils.CreateDirectory(cloneDirectory); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", cloneDirectory, err)
	}

	logs := new(bytes.Buffer)
	clone := exec.Command("git", "clone", "--no-checkout", repository.Repository, repository.Tag)
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
	checkout := exec.Command("git", "checkout", repository.Tag)
	checkout.Dir = gitDirectory
	checkout.Stdout = logs
	checkout.Stderr = logs

	if err := checkout.Run(); err != nil {
		return fmt.Errorf("failed to checkout to requested tag %q for repository %q: %w: %s", repository.Tag, repository.Repository, err, logs.String())
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
	var (
		targetDirectory = templateUtils.Templates{Directory: g.TargetDirectory}
		targetPath      = g.TemplatePath
		templatePath    = filepath.Join(g.ReadFromDirectory, targetPath, "provider.tpl")
	)

	file, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("error while reading template file %s : %w", templatePath, err)
	}

	tpl, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing template file %s : %w", templatePath, err)
	}

	fp := Fingerprint(targetPath)
	outputFile := fmt.Sprintf("%s-%s-provider-%s.tf", g.ID, data.Provider.SpecName, fp)
	if err := targetDirectory.Generate(tpl, outputFile, fingerPrintedData{
		Data:        data,
		Fingerprint: fp,
	}); err != nil {
		return fmt.Errorf("error while generating %s file : %w", outputFile, err)
	}

	if err := utils.CreateKeyFile(data.Provider.Credentials, g.TargetDirectory, data.Provider.SpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", data.Provider.SpecName, g.TargetDirectory, err)
	}

	return nil
}

func (g *Generator) GenerateNetworking(data *NetworkingData) error {
	var (
		targetDirectory = templateUtils.Templates{Directory: g.TargetDirectory}
		targetPath      = g.TemplatePath
		templatePath    = filepath.Join(g.ReadFromDirectory, targetPath, "networking.tpl")
		providerSpec    = data.Provider.SpecName
	)

	file, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("error while reading networking template file %s: %w", templatePath, err)
	}

	networking, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing networking_common template file %s : %w", templatePath, err)
	}

	fp := Fingerprint(targetPath)
	outputFile := fmt.Sprintf("%s-%s-networking-%s.tf", g.ID, providerSpec, fp)
	err = targetDirectory.Generate(networking, outputFile, fingerPrintedData{
		Data:        data,
		Fingerprint: fp,
	})
	if err != nil {
		return fmt.Errorf("error while generating %s file : %w", outputFile, err)
	}
	return nil
}

func (g *Generator) GenerateNodes(data *NodepoolsData) error {
	var (
		targetDirectory = templateUtils.Templates{Directory: g.TargetDirectory}
		targetPath      = g.TemplatePath
		networkingPath  = filepath.Join(g.ReadFromDirectory, targetPath, "node_networking.tpl")
		nodesPath       = filepath.Join(g.ReadFromDirectory, targetPath, "node.tpl")
		providerSpec    = data.NodePools[0].Details.GetProvider().GetSpecName()
	)

	file, err := os.ReadFile(networkingPath)
	if err == nil { // the template file might not exists
		networking, err := templateUtils.LoadTemplate(string(file))
		if err != nil {
			return fmt.Errorf("error while parsing node networking template file %s : %w", networkingPath, err)
		}

		fp := Fingerprint(targetPath)
		outputFile := fmt.Sprintf("%s-%s-node-networking-%s.tf", g.ID, providerSpec, fp)
		if err := targetDirectory.Generate(networking, outputFile, fingerPrintedData{
			Data:        data,
			Fingerprint: fp,
		}); err != nil {
			return fmt.Errorf("error while generating %s file : %w", outputFile, err)
		}
	}

	file, err = os.ReadFile(nodesPath)
	if err != nil {
		return fmt.Errorf("error while reading nodepool template file %s: %w", nodesPath, err)
	}

	nodepool, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing nodepool template file %s: %w", nodesPath, err)
	}

	fp := Fingerprint(targetPath)
	outputFile := fmt.Sprintf("%s-%s-nodepool-%s.tf", g.ID, providerSpec, fp)
	if err := targetDirectory.Generate(nodepool, outputFile, fingerPrintedData{
		Data:        data,
		Fingerprint: fp,
	}); err != nil {
		return fmt.Errorf("error while generating %s file: %w", outputFile, err)
	}
	return nil
}

func (g *Generator) GenerateDNS(data *DNSData) error {
	const dnsTemplate = "dns.tpl"

	var (
		targetDirectory = templateUtils.Templates{Directory: g.TargetDirectory}
		targetPath      = g.TemplatePath
		dnsPath         = filepath.Join(g.ReadFromDirectory, targetPath, dnsTemplate)
	)

	file, err := os.ReadFile(dnsPath)
	if err != nil {
		return fmt.Errorf("error while reading template file %s for %s : %w", dnsTemplate, g.TargetDirectory, err)
	}

	tpl, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing template file %s for %s : %w", dnsTemplate, g.TargetDirectory, err)
	}

	fp := Fingerprint(targetPath)
	outputfile := fmt.Sprintf("%s-%s-dns-%s.tf", g.ID, data.Provider.SpecName, fp)
	err = targetDirectory.Generate(tpl, outputfile, fingerPrintedData{
		Data:        data,
		Fingerprint: fp,
	})
	if err != nil {
		return fmt.Errorf("failed generating dns temaplate for %q: %w", g.ID, err)
	}

	if err := utils.CreateKeyFile(data.Provider.Credentials, g.TargetDirectory, data.Provider.SpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", data.Provider.SpecName, g.TargetDirectory, err)
	}

	return nil
}

func mustParseURL(s *url.URL, err error) *url.URL {
	if err != nil {
		panic(err)
	}
	return s
}

func ExtractTargetPath(repository *pb.TemplateRepository) string {
	u := mustParseURL(url.Parse(repository.Repository))
	return filepath.Join(
		u.Hostname(),
		u.Path,
		repository.Tag,
		repository.Path,
	)
}

func Fingerprint(s string) string {
	digest := sha512.Sum512_256([]byte(s))
	return hex.EncodeToString(digest[:16])
}
