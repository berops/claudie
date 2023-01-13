package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCmd tests a command retry and cancellation.
func TestCmd(t *testing.T) {
	//low commandTimeout - fail
	cmd1 := Cmd{"sleep 2 && ls", "", nil, nil, 1}
	err := cmd1.RetryCommand(1)
	require.Error(t, err)
	_, err = cmd1.RetryCommandWithOutput(1)
	require.Error(t, err)
	//high commandTimeout - pass
	cmd2 := Cmd{"sleep 2 && ls", "", nil, nil, 3}
	err = cmd2.RetryCommand(1)
	require.NoError(t, err)
	_, err = cmd2.RetryCommandWithOutput(1)
	require.NoError(t, err)
	//no commandTimeout - pass
	cmd3 := Cmd{"sleep 2 && ls", "", nil, nil, 0}
	err = cmd3.RetryCommand(1)
	require.NoError(t, err)
	_, err = cmd3.RetryCommandWithOutput(1)
	require.NoError(t, err)
}
