package extemplates

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

var (
	// ErrEmptyRepository is returned when no repository is to be cloned.
	ErrEmptyRepository = errors.New("no repository to clone")

	// ErrUnsupportedProtocol is returned when the protocol for the templates is not supported.
	ErrUnsupportedProtocol = errors.New("unsupported protocol for downloading templates")

	// ErrUnknownCommit is returned when checkout to the specified commit fails.
	ErrUnknownCommit = errors.New("failed to checkout to specified commit")
)

// VerifyCommitExists verify if the commit (whether a SHA hash, tag, annotated tag, branch etc...)
// is a valid object in the object database. If not the [ErrUnknownCommit] is returned.
func VerifyCommitExists(directory, commit string) error {
	logs := new(bytes.Buffer)

	// nolint
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", fmt.Sprintf("%s^{object}", commit))
	cmd.Dir = directory
	cmd.Stdout = logs
	cmd.Stderr = logs

	err := cmd.Run()
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return fmt.Errorf("%w %q: %w", ErrUnknownCommit, commit, err)
	}
	return fmt.Errorf("failed to check the existence of commit %q for %q: %w", commit, directory, err)
}

// Unsets worktree extension for git.
func UnsetWorktree(directory string) error {
	logs := new(bytes.Buffer)

	//nolint
	getWorktreeExtension := exec.Command("git", "config", "--local", "--get", "extensions.worktreeconfig")
	getWorktreeExtension.Dir = directory
	if err := getWorktreeExtension.Run(); err == nil {
		//nolint
		unsetWorktree := exec.Command("git", "config", "--local", "--unset", "extensions.worktreeconfig")
		unsetWorktree.Dir = directory
		unsetWorktree.Stdout = logs
		unsetWorktree.Stderr = logs
		if err := unsetWorktree.Run(); err != nil {
			return fmt.Errorf("failed to unset worktree extension %q: %w: %s", directory, err, logs.String())
		}
	}

	return nil
}

// Performs a sparse-checkout to only download the path and not the whole repository.
func SparseCheckout(directory string, path, commit string) error {
	logs := new(bytes.Buffer)

	//nolint
	sparseCheckout := exec.Command("git", "sparse-checkout", "set", path)
	sparseCheckout.Dir = directory
	sparseCheckout.Stdout = logs
	sparseCheckout.Stderr = logs

	if err := sparseCheckout.Run(); err != nil {
		return fmt.Errorf("failed to set sparse-checkout %q: %w: %s", path, err, logs.String())
	}

	logs.Reset()

	//nolint
	checkout := exec.Command("git", "checkout", commit)
	checkout.Dir = directory
	checkout.Stdout = logs
	checkout.Stderr = logs

	if err := checkout.Run(); err != nil {
		return fmt.Errorf("git checkout failed for %q, repository %q: %w: %s", commit, directory, err, logs.String())
	}

	return nil
}
