package terraform

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
)

const (
	// maxTfCommandRetryCount is the maximum amount a Terraform command can be repeated until
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

	// Parallelism is the number of resources to be worked on in parallel by terraform.
	Parallelism int

	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values must be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

func (t *Terraform) Init() error {
	t.SpawnProcessLimit <- struct{}{}
	defer func() { <-t.SpawnProcessLimit }()

	cmd := exec.Command("terraform", "init")
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "terraform init",
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
	t.SpawnProcessLimit <- struct{}{}
	defer func() { <-t.SpawnProcessLimit }()

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"apply",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	cmd := exec.Command("terraform", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("terraform %s", strings.Join(args, " "))

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
	t.SpawnProcessLimit <- struct{}{}
	defer func() { <-t.SpawnProcessLimit }()

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"destroy",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	cmd := exec.Command("terraform", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("terraform %s", strings.Join(args, " "))

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
	t.SpawnProcessLimit <- struct{}{}
	defer func() { <-t.SpawnProcessLimit }()

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

	cmd := exec.Command("terraform", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("terraform %s", strings.Join(args, " "))

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
	cmd := exec.Command("terraform", "state", "list")
	cmd.Dir = t.Directory
	out, err := cmd.Output()
	if err != nil {
		command := fmt.Sprintf("terraform state list")
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)
		retryCmd := comm.Cmd{
			Command: command,
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
	cmd := exec.Command("terraform", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.CombinedOutput()
	return string(out), err
}
