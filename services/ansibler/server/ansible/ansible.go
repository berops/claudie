package ansible

import (
	"fmt"
	"os"
	"os/exec"

	comm "github.com/berops/claudie/internal/command"
	"github.com/rs/zerolog/log"
)

const (
	defaultAnsibleForks = 30
	maxAnsibleRetries   = 10
)

type Ansible struct {
	Playbook  string
	Inventory string
	Flags     string
	Directory string
}

// RunAnsiblePlaybook executes ansible-playbook with the default forks of defaultAnsibleForks
// any additional flags like -l <name>, or --extra-vars <vars> include in flags parameter
// if command unsuccessful, the function will retry it until successful or maxAnsibleRetries reached
// all commands are executed with ANSIBLE_HOST_KEY_CHECKING set to false
func (a *Ansible) RunAnsiblePlaybook(prefix string) error {
	err := setEnv()
	if err != nil {
		return err
	}
	command := fmt.Sprintf("ansible-playbook %s -i %s -f %d %s", a.Playbook, a.Inventory, defaultAnsibleForks, a.Flags)
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = a.Directory
	cmd.Stdout = comm.GetStdOut(prefix)
	cmd.Stderr = comm.GetStdErr(prefix)
	err = cmd.Run()
	if err != nil {
		log.Warn().Msgf("Error encountered while executing %s from %s : %v", command, a.Directory, err)
		retryCmd := comm.Cmd{Command: command, Dir: a.Directory, Stdout: cmd.Stdout, Stderr: cmd.Stderr}
		err := retryCmd.RetryCommand(maxAnsibleRetries)
		if err != nil {
			return err
		}
	}
	return nil
}

// setEnv function will set environment variable to the environment before executing ansible
func setEnv() error {
	if err := os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False"); err != nil {
		return fmt.Errorf("failed to set ANSIBLE_HOST_KEY_CHECKING environment variable to False : %w", err)
	}
	return nil
}
