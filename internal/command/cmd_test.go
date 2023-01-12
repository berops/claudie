package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCmd(t *testing.T) {
	cmd := Cmd{"sleep 2 && ls", "", nil, nil}
	err := cmd.RetryCommand(1, 1)
	require.Error(t, err)
	err = cmd.RetryCommand(1, 3)
	require.NoError(t, err)
	_, err = cmd.RetryCommandWithOutput(1, 1)
	require.Error(t, err)
	_, err = cmd.RetryCommandWithOutput(1, 3)
	require.NoError(t, err)
}
