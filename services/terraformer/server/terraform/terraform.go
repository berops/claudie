package terraform

import (
	"bytes"
	"os"
	"os/exec"
)

// directory - directory of .tf files
type Terraform struct {
	Directory string
}

func (t Terraform) TerraformInit() error {
	cmd := exec.Command("terraform", "init")
	cmd.Dir = t.Directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t Terraform) TerraformApply() error {
	cmd := exec.Command("terraform", "apply", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func (t Terraform) TerraformDestroy() error {
	cmd := exec.Command("terraform", "destroy", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t Terraform) TerraformOutput(resourceName string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	var outb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outb.String(), nil
}
