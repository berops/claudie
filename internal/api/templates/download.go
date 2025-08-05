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
	"github.com/go-git/go-git/v5"
)

// ErrEmptyRepository is returned when no repository is to be cloned.
var ErrEmptyRepository = errors.New("no repository to clone")

type Repository struct {
	// URL of the git repository to download.
	Repository string
	// RootDir within within the git repository.
	Path string
	// Commit to checkout
	Commit     string
	CommitHash string
}

func Download(into string, repository Repository) error {
	if err := download(into, repository); err != nil {
		if errors.Is(err, ErrEmptyRepository) {
			msg := fmt.Sprintf("%q not found", repository.Repository)
			return fmt.Errorf("%s: %w", msg, err)
		}
		return err
	}
	return nil
}

func download(dir string, repository Repository) error {
	if repository.Repository == "" {
		return ErrEmptyRepository
	}

	u, err := url.Parse(repository.Repository)
	if err != nil {
		return fmt.Errorf("%s is not a valid url: %w", repository.Repository, err)
	}

	cloneDirectory := filepath.Join(dir, u.Hostname(), u.Path)
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

			//nolint
			sparseCheckout := exec.Command("git", "sparse-checkout", "set", strings.Trim(repository.Path, "/"))
			sparseCheckout.Dir = gitDirectory
			sparseCheckout.Stdout = logs
			sparseCheckout.Stderr = logs

			if err := sparseCheckout.Run(); err != nil {
				return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", repository.Repository, err, logs.String())
			}

			logs.Reset()

			// TODO: will we allow this to be empty ?
			args := []string{"checkout", repository.Commit}

			//nolint
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
	//nolint
	clone := exec.Command("git", "clone", "--no-checkout", repository.Repository, repository.CommitHash)
	clone.Dir = cloneDirectory
	clone.Stdout = logs
	clone.Stderr = logs

	if err := clone.Run(); err != nil {
		return fmt.Errorf("failed to clone %q: %w: %s", repository.Repository, err, logs.String())
	}

	logs.Reset()
	//nolint
	sparseCheckout := exec.Command("git", "sparse-checkout", "set", strings.Trim(repository.Path, "/"))
	sparseCheckout.Dir = gitDirectory
	sparseCheckout.Stdout = logs
	sparseCheckout.Stderr = logs

	if err := sparseCheckout.Run(); err != nil {
		return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", repository.Repository, err, logs.String())
	}

	logs.Reset()

	//nolint
	checkout := exec.Command("git", "checkout", repository.CommitHash)
	checkout.Dir = gitDirectory
	checkout.Stdout = logs
	checkout.Stderr = logs

	if err := checkout.Run(); err != nil {
		return fmt.Errorf("failed to checkout for %q, repository %q: %w: %s", repository.CommitHash, repository.Repository, err, logs.String())
	}

	return nil
}
