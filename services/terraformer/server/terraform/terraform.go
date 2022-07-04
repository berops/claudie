package terraform

import (
	"io"
	"os/exec"
)

// directory - directory of .tf files
type Terraform struct {
	Directory string
	StdOut    io.Writer
	StdErr    io.Writer
}

func (t Terraform) TerraformInit() error {
	cmd := exec.Command("terraform", "init")
	cmd.Dir = t.Directory
	cmd.Stdout = t.StdOut
	cmd.Stderr = t.StdErr
	return cmd.Run()
}

func (t Terraform) TerraformApply() error {
	cmd := exec.Command("terraform", "apply", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = t.StdOut
	cmd.Stderr = t.StdErr
	return cmd.Run()
}
func (t Terraform) TerraformDestroy() error {
	cmd := exec.Command("terraform", "destroy", "--auto-approve")
	cmd.Dir = t.Directory
	cmd.Stdout = t.StdOut
	cmd.Stderr = t.StdErr
	return cmd.Run()
}

func (t Terraform) TerraformOutput(resourceName string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", resourceName)
	cmd.Dir = t.Directory
	out, err := cmd.CombinedOutput()
	return string(out), err
}
