package main

import (
	"os"
	"os/exec"

	"github.com/Berops/platform/proto/pb"
)

const playbookPath = "./ansible/playbook.yml"
const inventoryPath = "inventory/inventory.ini"
const privateKeyPath = "/Users/samuelstolicny/.ssh/samuelstolicny_ssh_key"

func runAnsible(p *pb.Project) error {
	cmd := exec.Command(
		"ansible-playbook",
		playbookPath,
		"-i",
		inventoryPath,
		"-f",
		"20",
		"--private-key",
		p.GetCluster().GetPrivateKey(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
