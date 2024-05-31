package templates

import (
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

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

	repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:               repository.Repository,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %q: %w", repository.Repository, err)
	}

	tag, err := repo.Tag(repository.Tag)
	if err != nil {
		return fmt.Errorf("repository %q does not have tag %q: %w", repository.Repository, repository.Tag, err)
	}

	cloneDirectory := filepath.Join(
		r.TemplatesRootDirectory,
		u.Hostname(),
		u.Path,
		repository.Tag,
	)

	if utils.DirectoryExists(cloneDirectory) {
		if existingMirror, err := git.PlainOpen(cloneDirectory); err == nil {
			if err != nil {
				return fmt.Errorf("%q is not a valid local git repository: %w", cloneDirectory, err)
			}

			wk, err := existingMirror.Worktree()
			if err != nil {
				return fmt.Errorf("failed to acquire existing worktree for %q: %w", repository.Repository, err)
			}

			if err := wk.Checkout(&git.CheckoutOptions{Hash: tag.Hash()}); err == nil {
				return nil
			}
		}

		// localMirror does not have the required tag, overwrite with requested version
		if err := os.RemoveAll(cloneDirectory); err != nil {
			return fmt.Errorf("failed to delete local clone %q: %w", cloneDirectory, err)
		}
		// fallthrough, continue with the cloning below

	}

	localMirror, err := r.clone(cloneDirectory, repo)
	if err != nil {
		return fmt.Errorf("failed to create local copy of repository %q: %w", repository.Repository, err)
	}

	wk, err := localMirror.Worktree()
	if err != nil {
		return fmt.Errorf("failed to acquire worktree for %q: %w", repository.Repository, err)
	}

	if err := wk.Checkout(&git.CheckoutOptions{Hash: tag.Hash()}); err != nil {
		return fmt.Errorf("failed to checkout to the desired tag %q for %q: %w", repository.Tag, repository.Repository, err)
	}

	return nil
}

func (r *Repository) clone(dir string, upstream *git.Repository) (*git.Repository, error) {
	if err := utils.CreateDirectory(dir); err != nil {
		return nil, fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	localMirror, err := git.PlainInit(dir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize local repository %q: %w", dir, err)
	}

	objectIter, err := upstream.Storer.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream object iterator for %q: %w", dir, err)
	}

	err = objectIter.ForEach(func(eo plumbing.EncodedObject) error {
		_, err := localMirror.Storer.SetEncodedObject(eo)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over upstream objects for %q: %w", dir, err)
	}

	refsIter, err := upstream.Storer.IterReferences()
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream refs iterator for %q: %w", dir, err)
	}

	err = refsIter.ForEach(func(r *plumbing.Reference) error {
		return localMirror.Storer.SetReference(r)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over upstream refs for %q: %w", dir, err)
	}

	return localMirror, nil
}

type NodepoolGenerator struct {
	ClusterID string
	Nodepool  *pb.NodePool
	// Where the templates should be generated.
	TargetDirectory string
	// Root directory where the template files were downloaded
	// To this directory the relative path of the templates will
	// be added to read the templates for each nodepool.
	ReadFromDirectory string
}

func (g *NodepoolGenerator) GenerateProvider(data *ProviderData) error {
	var (
		targetDirectory  = templateUtils.Templates{Directory: g.TargetDirectory}
		providerSpecName = data.Provider.SpecName
	)

	targetPath := extractTargetPath(g.Nodepool.GetDynamicNodePool().GetTemplates())
	templatePath := filepath.Join(
		g.ReadFromDirectory,
		targetPath,
		"provider.tpl",
	)

	file, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("error while reading template file %s : %w", templatePath, err)
	}

	tpl, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing template file %s : %w", templatePath, err)
	}

	outputFile := fmt.Sprintf("%s-%s-provider-%s.tf", g.ClusterID, providerSpecName, fingerprint(targetPath))
	if err := targetDirectory.Generate(tpl, outputFile, data); err != nil {
		return fmt.Errorf("error while generating %s file : %w", outputFile, err)
	}

	if err := utils.CreateKeyFile(data.Provider.Credentials, g.TargetDirectory, providerSpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", providerSpecName, g.TargetDirectory, err)
	}

	return nil
}

func (g *NodepoolGenerator) GenerateNetworkingCommon(data *ProviderData) error {
	var (
		targetDirectory  = templateUtils.Templates{Directory: g.TargetDirectory}
		providerSpecName = data.Provider.SpecName
	)

	targetPath := extractTargetPath(g.Nodepool.GetDynamicNodePool().GetTemplates())
	templatePath := filepath.Join(
		g.ReadFromDirectory,
		targetPath,
		"networking.tpl",
	)

	file, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("error while reading networking template file %s: %w", templatePath, err)
	}

	networking, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing networking_common template file %s : %w", templatePath, err)
	}

	outputFile := fmt.Sprintf("%s-%s-networking-%s.tf", g.ClusterID, providerSpecName, fingerprint(targetPath))
	err = targetDirectory.Generate(networking, outputFile, data)
	if err != nil {
		return fmt.Errorf("error while generating %s file : %w", outputFile, err)
	}
	return nil
}

func (g *NodepoolGenerator) GenerateNodes(data *NodepoolsData, providerData *ProviderData) error {
	var (
		targetDirectory  = templateUtils.Templates{Directory: g.TargetDirectory}
		providerSpecName = providerData.Provider.SpecName
	)

	targetPath := extractTargetPath(g.Nodepool.GetDynamicNodePool().GetTemplates())
	networkingPath := filepath.Join(
		g.ReadFromDirectory,
		targetPath,
		"node_networking.tpl",
	)

	nodesPath := filepath.Join(
		g.ReadFromDirectory,
		targetPath,
		"node.tpl",
	)

	file, err := os.ReadFile(networkingPath)
	if err == nil { // the template file might not exists
		networking, err := templateUtils.LoadTemplate(string(file))
		if err != nil {
			return fmt.Errorf("error while parsing node networking template file %s : %w", networkingPath, err)
		}

		outputFile := fmt.Sprintf("%s-%s-node-networking-%s.tf", g.ClusterID, providerSpecName, fingerprint(targetPath))
		if err := targetDirectory.Generate(networking, outputFile, data); err != nil {
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

	outputFile := fmt.Sprintf("%s-%s-nodepool-%s.tf", g.ClusterID, providerSpecName, fingerprint(targetPath))
	if err := targetDirectory.Generate(nodepool, outputFile, data); err != nil {
		return fmt.Errorf("error while generating %s file: %w", outputFile, err)
	}
	return nil
}

type DNSGenerator struct {
	// Where the templates should be generated.
	TargetDirectory string
	// Root directory where the template files were downloaded
	// To this directory the relative path of the templates will
	// be added to read the templates for each nodepool.
	ReadFromDirectory string
	// DNS belonging to a LoadBalancer
	DNS *pb.DNS
}

func (g *DNSGenerator) GenerateDNS(dns *DNSData) error {
	const dnsTemplate = "dns.tpl"

	var (
		targetDirectory = templateUtils.Templates{Directory: g.TargetDirectory}
		targetPath      = extractTargetPath(g.DNS.Templates)
		dnsPath         = filepath.Join(
			g.ReadFromDirectory,
			targetPath,
			dnsTemplate,
		)
	)

	file, err := os.ReadFile(dnsPath)
	if err != nil {
		return fmt.Errorf("error while reading template file %s for %s : %w", dnsTemplate, g.TargetDirectory, err)
	}

	tpl, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing template file %s for %s : %w", dnsTemplate, g.TargetDirectory, err)
	}

	if err := utils.CreateKeyFile(dns.Provider.Credentials, g.TargetDirectory, dns.Provider.SpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", dns.Provider.SpecName, g.TargetDirectory, err)
	}

	outputFile := fmt.Sprintf("%s-dns-%s.tf", dns.Provider.CloudProviderName, fingerprint(targetPath))
	if err := targetDirectory.Generate(tpl, outputFile, dns); err != nil {
		return fmt.Errorf("failed generating dns template %q: %w", outputFile, err)
	}

	return nil
}

func mustParseURL(s *url.URL, err error) *url.URL {
	if err != nil {
		panic(err)
	}
	return s
}

func extractTargetPath(repository *pb.TemplateRepository) string {
	u := mustParseURL(url.Parse(repository.Repository))
	return filepath.Join(
		u.Hostname(),
		u.Path,
		repository.Tag,
		repository.Path,
	)
}

func fingerprint(s string) string {
	digest := sha512.Sum512_256([]byte(s))
	return hex.EncodeToString(digest[:16])
}
