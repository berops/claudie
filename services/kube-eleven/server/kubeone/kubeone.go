package kubeone

import (
	"os/exec"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
)

const (
	maxNumOfRetries = 5 //max number of retries for kubeone apply
)

// Kubeone struct
// Directory - directory where kubeone.yaml is located
type Kubeone struct {
	Directory string
}

// Apply will run `kubeone apply -m kubeone.yaml -y` in the specified directory
// return nil if successful, error otherwise
func (k *Kubeone) Apply(prefix string) error {
	command := "kubeone apply -m kubeone.yaml -y"
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.Directory
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		cmd.Stdout = comm.GetStdOut(prefix)
		cmd.Stderr = comm.GetStdErr(prefix)
	}
	if err := cmd.Run(); err != nil {
		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)
		retryCmd := comm.Cmd{
			Command: command,
			Dir:     k.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}
		if err := retryCmd.RetryCommand(maxNumOfRetries); err != nil {
			return err
		}
	}
	return nil
}
