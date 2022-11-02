package terraform

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	comm "github.com/Berops/claudie/internal/command"
	"github.com/rs/zerolog/log"
)

const (
	// maxTerraformRetries is the maximum amount a command can be repeated until
	// it succeeds. If after 10 retries the commands still fails an error should be
	// returned containing the reason.
	maxTerraformRetries = 10
)

type Terraform struct {
	// Directory represents the directory of .tf files
	Directory string
	StdOut    io.Writer
	StdErr    io.Writer
}

func (t *Terraform) TerraformInit() error {
	if err := setCache("."); err != nil {
		// log the warning but continue executions
		// error is not process breaking
		log.Warn().Msgf("Could not set cache for terraform plugins: %v", err)
	}

	cmd := exec.Command("terraform", "init")
	cmd.Dir = t.Directory
	cmd.Stdout = t.StdOut
	cmd.Stderr = t.StdErr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "terraform init",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTerraformRetries); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) TerraformApply() error {
	cmd := exec.Command("terraform", "apply", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = t.StdOut
	cmd.Stderr = t.StdErr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "terraform apply --auto-approve",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTerraformRetries); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) TerraformDestroy() error {
	cmd := exec.Command("terraform", "destroy", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = t.StdOut
	cmd.Stderr = t.StdErr

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: "terraform destroy --auto-approve",
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTerraformRetries); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t Terraform) TerraformOutput(resourceName string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (t Terraform) TerraformProvidersMirror(dir string) error {
	cmd := exec.Command("terraform", "providers", "mirror", dir)
	cmd.Dir = t.Directory

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: fmt.Sprintf("terraform providers mirror %s", dir),
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxTerraformRetries); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}
	return nil
}

// setCache function will set environment variable to the environment before executing terraform
// function also checks if cache directory for plugins exists and creates one if not
func setCache(cacheDir string) error {
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err = os.MkdirAll(cacheDir, 0777); err != nil {
			return fmt.Errorf("failed to create cache dir %s", cacheDir)
		}
	}
	if err := os.Setenv("TF_PLUGIN_CACHE_DIR", cacheDir); err != nil {
		return fmt.Errorf("failed to set TF_PLUGIN_CACHE_DIR env var as %s: %w", cacheDir, err)
	}
	return nil
}
