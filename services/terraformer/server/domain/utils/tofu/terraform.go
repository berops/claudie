package tofu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

const (
	// maxTfCommandRetryCount is the maximum amount a Tofu command can be repeated until
	// it succeeds. If after "maxTfCommandRetryCount" retries the commands still fails an error should be
	// returned containing the reason.
	maxTfCommandRetryCount = 3

	// Parallelism is the number of resource to be work on in parallel during the apply/destroy commands.
	Parallelism = 8
)

type Terraform struct {
	// Directory represents the directory of .tf files
	Directory string

	Stdout io.Writer
	Stderr io.Writer

	// Parallelism is the number of resources to be worked on in parallel by tofu.
	Parallelism int

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (t *Terraform) Init() error {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu init process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	//nolint
	cmd := exec.Command("tofu", "init")
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "tofu init",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Apply() error {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu apply process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"apply",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("tofu %s", strings.Join(args, " "))

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Destroy() error {
	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu destroy process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"destroy",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("tofu %s", strings.Join(args, " "))

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) DestroyTarget(targets []string) error {
	if len(targets) == 0 {
		return nil
	}

	if err := t.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare tofu destroy target process: %w", err)
	}
	defer t.SpawnProcessLimit.Release(1)

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"destroy",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	for _, resource := range targets {
		args = append(args, fmt.Sprintf("--target=%s", resource))
	}

	//nolint
	cmd := exec.Command("tofu", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("tofu %s", strings.Join(args, " "))

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		// NOTE: the maxTfCommandRetryCount * 2 is crucial here. Some resources may have a kind of
		// "lock" on a resource that cannot be immediately deleted and a timeout is needed, for example
		// this is the case with azures NIC which have a reservation for 180.
		if err := retryCmd.RetryCommand(maxTfCommandRetryCount * 2); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) StateList() ([]string, error) {
	//nolint
	cmd := exec.Command("tofu", "state", "list")
	cmd.Dir = t.Directory
	out, err := cmd.Output()
	if err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)
		retryCmd := comm.Cmd{
			Command: "tofu state list",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}
		if err := retryCmd.RetryCommand(maxTfCommandRetryCount); err != nil {
			return nil, fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
		return nil, err
	}

	r := bytes.Split(out, []byte("\n"))
	var resources []string
	for _, b := range r {
		if r := strings.TrimSpace(string(b)); r != "" {
			resources = append(resources, strings.TrimSpace(string(b)))
		}
	}

	return resources, nil
}

func (t *Terraform) Output(resourceName string) (string, error) {
	//nolint
	cmd := exec.Command("tofu", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.CombinedOutput()
	return string(out), err
}
