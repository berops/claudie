package main

import (
	"os"
	"os/exec"
)

const playbook_path = "/Users/samuelstolicny/Github/Berops/platform/wireguardian/ansible/playbook.yml"
const inventory_path = "inventory/inventory.ini"
const private_key_path = "/Users/samuelstolicny/.ssh/samuelstolicny_ssh_key"

func runAnsible() error {
	//ansible-playbook -i inventory.ini playbook.yml -f 20 --private-key ~/.ssh/samuelstolicny_ssh_key
	cmd := exec.Command(
		"ansible-playbook",
		playbook_path,
		"-i",
		inventory_path,
		"-f",
		"20",
		"--private-key",
		private_key_path,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}