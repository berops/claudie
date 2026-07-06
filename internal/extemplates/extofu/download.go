package extofu

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/extemplates"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/go-git/go-git/v5"
)

// Downloads the templates needed for the terraformer service into the specified directory.
//
// The templates will be stored under the path as returned by [TemplatesPath] inside the `donwloadInto`
// directory.
func Download(downloadInto string, provider *spec.Provider) error {
	if provider.GetTemplates() == nil {
		return extemplates.ErrEmptyRepository
	}

	var endpoint string
	switch provider.Templates.Endpoint.Protocol {
	case spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS:
		endpoint = provider.Templates.HttpsUrl()
	default:
		return fmt.Errorf("%v: %w", provider.Templates.Endpoint.Protocol, extemplates.ErrUnsupportedProtocol)
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("%s is not a valid url: %w", endpoint, err)
	}

	var (
		cloneDirectory    = filepath.Join(downloadInto, u.Hostname(), u.Path)
		gitDirectory      = filepath.Join(cloneDirectory, provider.Templates.CommitHash)
		providerTemplates = strings.Trim(filepath.Join(provider.Templates.Paths.Terraformer, provider.CloudProviderName), string(filepath.Separator))
	)

	if fileutils.DirectoryExists(gitDirectory) {
		// Worktree is not supported by go-git, thus check if worktree is set.
		if err := extemplates.UnsetWorktree(gitDirectory); err != nil {
			return err
		}

		existingMirror, err := git.PlainOpen(gitDirectory)
		if err != nil {
			return fmt.Errorf("%q is not a valid local git repository: %w", gitDirectory, err)
		}

		ref, err := existingMirror.Head()
		if err != nil {
			return fmt.Errorf("failed to read HEAD of local repository %q: %w", gitDirectory, err)
		}

		if ref.Hash().String() == provider.Templates.CommitHash {
			if err := extemplates.SparseCheckout(gitDirectory, providerTemplates, provider.Templates.CommitHash); err != nil {
				return err
			}
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
	//nolint
	clone := exec.Command("git", "clone", "--no-checkout", endpoint, provider.Templates.CommitHash)
	clone.Dir = cloneDirectory
	clone.Stdout = logs
	clone.Stderr = logs

	if err := clone.Run(); err != nil {
		return fmt.Errorf("failed to clone %q: %w: %s", endpoint, err, logs.String())
	}

	if err := extemplates.VerifyCommitExists(gitDirectory, provider.Templates.Commit); err != nil {
		return fmt.Errorf("commit verification failed: %w", err)
	}

	if err := extemplates.SparseCheckout(gitDirectory, providerTemplates, provider.Templates.CommitHash); err != nil {
		return err
	}
	// Worktree is not supported by go-git, thus check if worktree is set.
	return extemplates.UnsetWorktree(gitDirectory)
}
