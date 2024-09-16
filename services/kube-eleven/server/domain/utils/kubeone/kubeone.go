package kubeone

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
)

// maxRetryCount is max number of retries for kubeone apply.
const maxRetryCount = 2

type Kubeone struct {
	// ConfigDirectory is the directory where the generated kubeone.yaml will be located.
	ConfigDirectory string
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned kubeone
	// processes. This values must be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

func (k *Kubeone) Reset(prefix string) error {
	k.SpawnProcessLimit <- struct{}{}
	defer func() { <-k.SpawnProcessLimit }()

	output := new(bytes.Buffer)

	command := fmt.Sprintf("kubeone reset -m kubeone.yaml -y --remove-binaries %s", structuredLogging())
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.ConfigDirectory
	cmd.Stdout = output
	cmd.Stderr = output

	if log.Logger.GetLevel() <= zerolog.InfoLevel {
		cmd.Stdout = comm.GetStdOut(prefix)
		cmd.Stderr = comm.GetStdErr(prefix)
	}

	if err := cmd.Run(); err != nil {
		l, errParse := collectErrors(output)
		if errParse == nil && len(l) > 0 {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, l.prettyPrint())
		}
		if errParse != nil {
			log.Warn().Msgf("failed to parse errors from kubeone logs: %v", errParse)
		}

		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     k.ConfigDirectory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		err = retryCmd.RetryCommandWithCallback(maxRetryCount, func() error {
			l, errParse := collectErrors(output)
			if errParse != nil {
				output.Reset()
				log.Warn().Msgf("failed to parse errors from kubeone logs: %v", errParse)
				return nil
			}
			if len(l) > 0 {
				log.Error().Msgf("failed to execute cmd: %s: %s", retryCmd.Command, l.prettyPrint())
			}
			output.Reset()
			return nil
		})

		if err != nil {
			l, errParse := collectErrors(output)
			if errParse != nil {
				log.Warn().Msgf("failed to parse errors from kubeone logs: %v", errParse)
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

// Apply will run `kubeone apply -m kubeone.yaml -y` in the ConfigDirectory.
// Returns nil if successful, error otherwise.
func (k *Kubeone) Apply(prefix string) error {
	k.SpawnProcessLimit <- struct{}{}
	defer func() { <-k.SpawnProcessLimit }()

	output := new(bytes.Buffer)

	command := fmt.Sprintf("kubeone apply -m kubeone.yaml -y %s", structuredLogging())
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.ConfigDirectory
	cmd.Stdout = output
	cmd.Stderr = output

	if log.Logger.GetLevel() <= zerolog.InfoLevel {
		// Here prefix is the cluster id
		cmd.Stdout = comm.GetStdOut(prefix)
		cmd.Stderr = comm.GetStdErr(prefix)
	}

	if err := cmd.Run(); err != nil {
		l, errParse := collectErrors(output)
		if errParse == nil && len(l) > 0 {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, l.prettyPrint())
		}
		if errParse != nil {
			log.Warn().Msgf("failed to parse errors from kubeone logs: %v", errParse)
		}

		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     k.ConfigDirectory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}
		err = retryCmd.RetryCommandWithCallback(maxRetryCount, func() error {
			l, errParse := collectErrors(output)
			if errParse != nil {
				output.Reset()
				log.Warn().Msgf("failed to parse errors from kubeone logs: %v", errParse)
				return nil
			}
			if len(l) > 0 {
				log.Error().Msgf("failed to execute cmd: %s: %s", retryCmd.Command, l.prettyPrint())
			}
			output.Reset()
			return nil
		})

		if err != nil {
			l, errParse := collectErrors(output)
			if errParse != nil {
				log.Warn().Msgf("failed to parse errors from kubeone logs: %v", errParse)
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

func structuredLogging() string {
	if log.Logger.GetLevel() <= zerolog.InfoLevel {
		return ""
	}
	return "--log-format json"
}
