package kubeone

import (
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"

	comm "github.com/Berops/claudie/internal/command"
)

const (
	maxNumOfRetries = 3 //max number of retries for kubeone apply
)

// Kubeone struct
// Directory - directory where kubeone.yaml is located
type Kubeone struct {
	Directory string
}

//Apply will run `kubeone apply -m kubeone.yaml -y` in the specified directory
//return nil if successful, error otherwise
func (k *Kubeone) Apply() error {
	command := "kubeone apply -m kubeone.yaml -y"
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = k.Directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Warn().Msgf("Error encountered while executing %s : %v", command, err)
		retryCmd := comm.Cmd{Command: command, Dir: k.Directory}
		err := retryCmd.RetryCommand(maxNumOfRetries)
		if err != nil {
			return err
		}
	}
	return nil
}
