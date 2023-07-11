package terraform

import (
	"bytes"
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
	output := new(bytes.Buffer)

	cmd := exec.Command("terraform", "apply", "-json", "--auto-approve")
	cmd.Dir = t.Directory

	if cmd.Stdout == nil && cmd.Stderr == nil {
		cmd.Stdout = output
		cmd.Stderr = output
	}

	if err := cmd.Run(); err != nil {
		command := "terraform apply -json --auto-approve"

		l, err := collectErrors(output)
		if err == nil {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, l.prettyPrint())
		}
		if err != nil {
			log.Warn().Msgf("failed to parse errors from terraform logs: %v", err)
		}
		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		err = retryCmd.RetryCommandWithCallback(maxTfCommandRetryCount, func() error {
			l, err := collectErrors(output)
			if err != nil {
				output.Reset()
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", err)
				return nil
			}
			log.Error().Msgf("failed to execute cmd: %s: %s", retryCmd.Command, l.prettyPrint())
			output.Reset()
			return nil
		})

		if err != nil {
			l, err2 := collectErrors(output)
			if err2 != nil {
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", err2)
				return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
			}

			err = fmt.Errorf("%w: %s", err, l.prettyPrint())
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Destroy() error {
	output := new(bytes.Buffer)

	cmd := exec.Command("terraform", "destroy", "-json", "--auto-approve")
	cmd.Dir = t.Directory

	if cmd.Stdout == nil && cmd.Stderr == nil {
		cmd.Stdout = output
		cmd.Stderr = output
	}

	if err := cmd.Run(); err != nil {
		command := "terraform destroy -json --auto-approve"

		l, err := collectErrors(output)
		if err == nil {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, l.prettyPrint())
		}
		if err != nil {
			log.Warn().Msgf("failed to parse errors from terraform logs: %v", err)
		}
		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		err = retryCmd.RetryCommandWithCallback(maxTfCommandRetryCount, func() error {
			l, err := collectErrors(output)
			if err != nil {
				output.Reset()
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", err)
				return nil
			}
			log.Error().Msgf("failed to execute cmd: %s: %s", retryCmd.Command, l.prettyPrint())
			output.Reset()
			return nil
		})

		if err != nil {
			l, err2 := collectErrors(output)
			if err2 != nil {
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", err2)
				return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
			}

			err := fmt.Errorf("%w: %s", err, l.prettyPrint())
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
