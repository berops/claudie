package extofu

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/extemplates"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Downloads the templates needed for the terraformer service into the specified directory.
//
// The templates will be stored under the path as returned by [TemplatesPath] inside the `donwloadInto`
// directory.
//
// On empty template repository the [extemplates.ErrEmptyRepository] error is returned.
// On unsupported endpoint protocol the [extemplates.ErrUnsupportedProtocol] error is returned.
// On unknown commit the [extemplates.ErrUnknownCommit] error is returned.
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
		gitDirectory      = filepath.Join(downloadInto, u.Hostname(), u.Path, provider.Templates.CommitHash)
		providerTemplates = strings.Trim(filepath.Join(provider.Templates.Paths.Terraformer, provider.CloudProviderName), string(filepath.Separator))
	)

	if fileutils.DirectoryExists(gitDirectory) {
		existingMirror, err := git.PlainOpen(gitDirectory)
		if err != nil {
			return fmt.Errorf("%q is not a valid local git repository: %w", gitDirectory, err)
		}

		w, err := existingMirror.Worktree()
		if err != nil {
			return fmt.Errorf("failed to read worktree %q: %w", gitDirectory, err)
		}

		ref, err := existingMirror.Head()
		if err != nil {
			return fmt.Errorf("failed to read HEAD of local repository %q: %w", gitDirectory, err)
		}

		if h := plumbing.NewHash(provider.Templates.CommitHash); ref.Hash() == h {
			return w.Checkout(&git.CheckoutOptions{
				SparseCheckoutDirectories: []string{providerTemplates},
				Hash:                      h,
			})
		}

		// on mismatch re-download the repo.
		if err := os.RemoveAll(gitDirectory); err != nil {
			return fmt.Errorf("failed to delete local clone %q: %w", gitDirectory, err)
		}
		// fallthrough, continue with the cloning below
	}

	if err := fileutils.CreateDirectory(gitDirectory); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", gitDirectory, err)
	}

	opts := git.CloneOptions{
		URL:        endpoint,
		Auth:       nil,
		NoCheckout: true,
	}

	if provider.Templates.Auth != nil {
		auth := http.BasicAuth{
			Username: "x-access-token",
			Password: provider.Templates.Auth.Token,
		}
		if provider.Templates.Auth.Username != nil {
			auth.Username = *provider.Templates.Auth.Username
		}
		opts.Auth = &auth
	}

	r, err := git.PlainClone(gitDirectory, false, &opts)
	if err != nil {
		return fmt.Errorf("failed to clone %q: %w", endpoint, err)
	}

	h := plumbing.NewHash(provider.Templates.CommitHash)
	if _, err := r.CommitObject(h); err != nil {
		if !errors.Is(err, plumbing.ErrObjectNotFound) {
			return fmt.Errorf("failed to check existence of commit %q for %q: %w", provider.Templates.CommitHash, endpoint, err)
		}
		return fmt.Errorf("commit %q for %q does not exist: %w: %w", provider.Templates.CommitHash, endpoint, err, extemplates.ErrUnknownCommit)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to read worktree %q: %w", gitDirectory, err)
	}

	return w.Checkout(&git.CheckoutOptions{
		SparseCheckoutDirectories: []string{providerTemplates},
		Hash:                      h,
	})
}
