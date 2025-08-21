package templates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/go-git/go-git/v5"
)

var (
	// ErrValidationFailed is returned when the passed in [Repository] fails
	// to be validated.
	ErrValidationFailed = errors.New("invalid repository")
)

type Repository struct {
	// URL of the git repository to download.
	Repository string
	// parsed repository as [url.URL].
	url *url.URL
	// RootDir within within the git repository.
	Path string
	// Full commit hash
	CommitHash string
}

func (r *Repository) validate() error {
	if r.Repository == "" {
		return fmt.Errorf("%w: empty repository url", ErrValidationFailed)
	}

	u, err := url.Parse(r.Repository)
	if err != nil {
		return fmt.Errorf("%w: %s is not a valid url", ErrValidationFailed, r.Repository)
	}
	r.url = u

	if r.Path == "" {
		return fmt.Errorf("%w: empty path", ErrValidationFailed)
	}
	if r.CommitHash == "" {
		return fmt.Errorf("%w: empty commit hash", ErrValidationFailed)
	}

	return nil
}

// Downloads the git repository [Repository.Repository] using a sparse-checkout rooted at [Repository.Path]
// with the reference commit [Repository.CommitHash].
func Download(ctx context.Context, dir string, repository Repository) error {
	if err := repository.validate(); err != nil {
		return err
	}

	cloneDirectory := filepath.Join(dir, repository.url.Hostname(), repository.url.Path)
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

			sparseCheckout := exec.CommandContext(ctx, "git", "sparse-checkout", "set", strings.Trim(repository.Path, "/"))
			sparseCheckout.Dir = gitDirectory
			sparseCheckout.Stdout = logs
			sparseCheckout.Stderr = logs

			if err := sparseCheckout.Run(); err != nil {
				return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", repository.Repository, err, logs.String())
			}

			logs.Reset()

			args := []string{"checkout", repository.CommitHash}

			checkout := exec.CommandContext(ctx, "git", args...)
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

	clone := exec.CommandContext(ctx, "git", "clone", "--no-checkout", repository.Repository, repository.CommitHash)
	clone.Dir = cloneDirectory
	clone.Stdout = logs
	clone.Stderr = logs

	if err := clone.Run(); err != nil {
		return fmt.Errorf("failed to clone %q: %w: %s", repository.Repository, err, logs.String())
	}

	logs.Reset()

	sparseCheckout := exec.CommandContext(ctx, "git", "sparse-checkout", "set", strings.Trim(repository.Path, "/"))
	sparseCheckout.Dir = gitDirectory
	sparseCheckout.Stdout = logs
	sparseCheckout.Stderr = logs

	if err := sparseCheckout.Run(); err != nil {
		return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", repository.Repository, err, logs.String())
	}

	logs.Reset()

	checkout := exec.CommandContext(ctx, "git", "checkout", repository.CommitHash)
	checkout.Dir = gitDirectory
	checkout.Stdout = logs
	checkout.Stderr = logs

	if err := checkout.Run(); err != nil {
		return fmt.Errorf("failed to checkout for %q, repository %q: %w: %s", repository.CommitHash, repository.Repository, err, logs.String())
	}

	return nil
}
