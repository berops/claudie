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

// TestSanitisedCmd checks that sanitisedCmd method only sanitises the wanted
// cases.
func TestSanitisedCmd(t *testing.T) {
	testCmd := &Cmd{}
	testCases := []struct {
		desc string
		in   string
		out  string
	}{
		{
			desc: "Sanitise kubectl command",
			in:   "kubectl blah --kubeconfig 'not valid but needs obscuring' --more-stuff",
			out:  "kubectl blah --kubeconfig '*****' --more-stuff",
		},
		{
			desc: "Sanitise kubectl command with invalid kubeconfig (should not change)",
			in:   "kubectl stuff --kubeconfig --more-args",
			out:  "kubectl stuff --kubeconfig --more-args",
		},
		{
			desc: "Don't touch the arg to --kubeconfig for unknown commands",
			in:   "idontknowthisone --kubeconfig 'the real kubeconfig is here'",
			out:  "idontknowthisone --kubeconfig 'the real kubeconfig is here'",
		},
		{
			desc: "Don't touch the thing for unknown commands (even when the arg to --kubeconfig is invalid)",
			in:   "idontknowthisone --kubeconfig --forgot-the-kubeconfig",
			out:  "idontknowthisone --kubeconfig --forgot-the-kubeconfig",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			want := tC.out

			testCmd.Command = tC.in

			if got := testCmd.sanitisedCmd(); got != want {
				t.Errorf("Unexpected output for %q: expected %q, actual %q",
					tC.desc, want, got)
			}
		})
	}
}
