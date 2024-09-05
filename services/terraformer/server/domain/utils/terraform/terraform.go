package terraform

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog"
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

	output := new(bytes.Buffer)

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"apply",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	if log.Logger.GetLevel() != zerolog.DebugLevel {
		args = append(args, "-json")

		t.Stdout = output
		t.Stderr = output
	}

	cmd := exec.Command("terraform", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("terraform %s", strings.Join(args, " "))

		l, errParse := collectErrors(output)
		if errParse == nil {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, l.prettyPrint())
		}
		if errParse != nil {
			log.Warn().Msgf("failed to parse errors from terraform logs: %v", errParse)
		}
		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		err := retryCmd.RetryCommandWithCallback(maxTfCommandRetryCount, func() error {
			l, errParse := collectErrors(output)
			if errParse != nil {
				output.Reset()
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", errParse)
				return nil
			}
			log.Error().Msgf("failed to execute cmd: %s: %s", retryCmd.Command, l.prettyPrint())
			output.Reset()
			return nil
		})

		if err != nil {
			l, errParse := collectErrors(output)
			if errParse != nil {
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", errParse)
				return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
			}

			if len(l) > 0 {
				err = fmt.Errorf("%w: %s", err, l.prettyPrint())
			}
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Destroy() error {
	t.SpawnProcessLimit <- struct{}{}
	defer func() { <-t.SpawnProcessLimit }()

	output := new(bytes.Buffer)

	if t.Parallelism <= 0 {
		t.Parallelism = Parallelism
	}

	args := []string{
		"destroy",
		"--auto-approve",
		fmt.Sprintf("--parallelism=%v", t.Parallelism),
	}

	if log.Logger.GetLevel() != zerolog.DebugLevel {
		args = append(args, "-json")

		t.Stdout = output
		t.Stderr = output
	}

	cmd := exec.Command("terraform", args...)
	cmd.Dir = t.Directory
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr

	if err := cmd.Run(); err != nil {
		command := fmt.Sprintf("terraform %s", strings.Join(args, " "))

		l, errParse := collectErrors(output)
		if errParse == nil {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, l.prettyPrint())
		}
		if errParse != nil {
			log.Warn().Msgf("failed to parse errors from terraform logs: %v", errParse)
		}
		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", cmd, t.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     t.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		err := retryCmd.RetryCommandWithCallback(maxTfCommandRetryCount, func() error {
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
			l, errParse := collectErrors(output)
			if errParse != nil {
				log.Warn().Msgf("failed to parse errors from terraform logs: %v", errParse)
				return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
			}

			if len(l) > 0 {
				err = fmt.Errorf("%w: %s", err, l.prettyPrint())
			}
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

func (t *Terraform) Output(resourceName string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.CombinedOutput()
	return string(out), err
}
