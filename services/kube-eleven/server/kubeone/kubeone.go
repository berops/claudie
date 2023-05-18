package kubeone

import (
	"os/exec"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
)

const (
	// max number of retries for kubeone apply
	maxRetryCount = 5

	command = "kubeone apply -m kubeone.yaml -y"
)

type Kubeone struct {
	// ConfigDirectory is the directory where the generated kubeone.yaml will be located.
	ConfigDirectory string
}

// Apply will run `kubeone apply -m kubeone.yaml -y` in the ConfigDirectory.
// Returns nil if successful, error otherwise.
func (k *Kubeone) Apply(prefix string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.ConfigDirectory
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		// Here prefix is the cluster id
		cmd.Stdout = comm.GetStdOut(prefix)
		cmd.Stderr = comm.GetStdErr(prefix)
	}

	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     k.ConfigDirectory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}
		if err := retryCmd.RetryCommand(maxRetryCount); err != nil {
			return err
		}
	}
	return nil
}
