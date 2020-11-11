package main

import (
	"os"
	"os/exec"
)

const playbookPath = "./ansible/playbook.yml"
const inventoryPath = "inventory/inventory.ini"
const privateKeyPath = "/Users/samuelstolicny/.ssh/samuelstolicny_ssh_key"

func runAnsible() error {
	//ansible-playbook -i inventory.ini playbook.yml -f 20 --private-key ~/.ssh/samuelstolicny_ssh_key
	cmd := exec.Command(
		"ansible-playbook",
		playbookPath,
		"-i",
		inventoryPath,
		"-f",
		"20",
		"--private-key",
		privateKeyPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
