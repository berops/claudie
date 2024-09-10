package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/templateUtils"
)

const (
	// File name for the ansible inventory.
	InventoryFileName = "inventory.ini"
	// defaultAnsibleForks defines how many forks ansible uses (on how many nodes can ansible perform a task at the same time).
	defaultAnsibleForks = 15
	// maxAnsibleRetries defines how many times should be playbook retried before returning error.
	maxAnsibleRetries = 5
)

// GenerateInventoryFile generates an Ansible inventory file that defines
// the hosts and groups of hosts that Ansible can manage.
func GenerateInventoryFile(inventoryTemplate, outputDirectory string, data interface{}) error {
	template, err := templateUtils.LoadTemplate(inventoryTemplate)
	if err != nil {
		return fmt.Errorf("error while loading Ansible inventory template for %s : %w", outputDirectory, err)
	}

	err = templateUtils.Templates{Directory: outputDirectory}.
		Generate(template, InventoryFileName, data)
	if err != nil {
		return fmt.Errorf("error while generating from template for %s : %w", outputDirectory, err)
	}

	return nil
}

type Ansible struct {
	Playbook  string
	Inventory string
	Flags     string
	Directory string
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned ansible
	// processes. This values must be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

// RunAnsiblePlaybook executes ansible-playbook with the default forks of defaultAnsibleForks
// any additional flags like -l <name>, or --extra-vars <vars> include in flags parameter
// if command unsuccessful, the function will retry it until successful or maxAnsibleRetries reached
// all commands are executed with ANSIBLE_HOST_KEY_CHECKING set to false
func (a *Ansible) RunAnsiblePlaybook(prefix string) error {
	a.SpawnProcessLimit <- struct{}{}
	defer func() { <-a.SpawnProcessLimit }()

	if err := setEnv(); err != nil {
		return err
	}

	output := new(bytes.Buffer)

	command := fmt.Sprintf("ansible-playbook %s -i %s -f %d %s", a.Playbook, a.Inventory, defaultAnsibleForks, a.Flags)
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = a.Directory
	cmd.Stdout = output
	cmd.Stderr = output

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		cmd.Stdout = comm.GetStdOut(prefix)
		cmd.Stderr = comm.GetStdErr(prefix)
	}

	if err := cmd.Run(); err != nil {
		if errPlaybook := collectErrors(output); errPlaybook != nil {
			log.Error().Msgf("failed to execute cmd: %s: %s", command, errPlaybook)
		}
		output.Reset()

		log.Warn().Msgf("Error encountered while executing %s from %s: %v", command, a.Directory, err)

		retryCmd := comm.Cmd{
			Command: command,
			Dir:     a.Directory,
			Stdout:  cmd.Stdout,
			Stderr:  cmd.Stderr,
		}

		err := retryCmd.RetryCommandWithCallback(maxAnsibleRetries, func() error {
			if errPlaybook := collectErrors(output); errPlaybook != nil {
				log.Error().Msgf("failed to execute cmd: %s: %s", retryCmd.Command, errPlaybook)
			}
			output.Reset()
			return nil
		})

		if err != nil {
			if errPlaybook := collectErrors(output); errPlaybook != nil {
				err = fmt.Errorf("%w:%w", err, errPlaybook)
			}
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

	if log.Logger.GetLevel() != zerolog.DebugLevel {
		if err := os.Setenv("ANSIBLE_STDOUT_CALLBACK", "json"); err != nil {
			return fmt.Errorf("failed to set ANSIBLE_STDOUT_CALLBACK environment variable to json: %w", err)
		}
	}

	return nil
}
