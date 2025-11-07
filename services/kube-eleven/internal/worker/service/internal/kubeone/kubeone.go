package kubeone

import (
	"context"
	"fmt"
	"os/exec"

	comm "github.com/berops/claudie/internal/command"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

// maxRetryCount is max number of retries for kubeone apply.
const maxRetryCount = 1

type Kubeone struct {
	// ConfigDirectory is the directory where the generated kubeone.yaml will be located.
	ConfigDirectory string
	// SpawnProcessLimit limits the number of spawned kubeone processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (k *Kubeone) Reset(prefix string) error {
	if err := k.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare kubeone reset process: %w", err)
	}
	defer k.SpawnProcessLimit.Release(1)

	command := "kubeone reset -m kubeone.yaml -y --remove-binaries"
	//nolint
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.ConfigDirectory

	cmd.Stdout = comm.GetStdOut(prefix)
	cmd.Stderr = comm.GetStdErr(prefix)

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     k.ConfigDirectory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}

	return nil
}

// Apply will run `kubeone apply -m kubeone.yaml -y` in the ConfigDirectory.
// Returns nil if successful, error otherwise.
func (k *Kubeone) Apply(prefix string) error {
	if err := k.SpawnProcessLimit.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("failed to prepare kubeone apply process: %w", err)
	}
	defer k.SpawnProcessLimit.Release(1)

	command := "kubeone apply -m kubeone.yaml -y"
	//nolint
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.ConfigDirectory

	// Here prefix is the cluster id
	cmd.Stdout = comm.GetStdOut(prefix)
	cmd.Stderr = comm.GetStdErr(prefix)

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     k.ConfigDirectory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		if err := retryCmd.RetryCommand(maxRetryCount); err != nil {
			return fmt.Errorf("failed to execute cmd: %s: %w", retryCmd.Command, err)
		}
	}
	return nil
}
