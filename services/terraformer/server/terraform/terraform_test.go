package terraform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	clusterDir = "" //set this as a directory where terraform will run
	outputName = "" //set this as an output name
)

func TestOutputTerraform(t *testing.T) {
	terraform := Terraform{Directory: clusterDir}
	out, err := terraform.TerraformOutput(outputName)
	t.Log(out)
	require.NoError(t, err)
}
