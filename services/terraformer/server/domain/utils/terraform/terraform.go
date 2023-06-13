package terraform

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
)

const (
	// maxTfCommandRetryCount is the maximum amount a Terraform command can be repeated until
	// it succeeds. If after "maxTfCommandRetryCount" retries the commands still fails an error should be
	// returned containing the reason.
	maxTfCommandRetryCount = 5
)

type Terraform struct {
	// Directory represents the directory of .tf files
	Directory string

	Stdout io.Writer
	Stderr io.Writer
}

func (t *Terraform) Init() error {
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
	cmd := exec.Command("terraform", "apply", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "terraform apply --auto-approve",
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
	cmd := exec.Command("terraform", "destroy", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "terraform destroy --auto-approve",
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

func (t Terraform) Output(resourceName string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.CombinedOutput()
	return string(out), err
}
