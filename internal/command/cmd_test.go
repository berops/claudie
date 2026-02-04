package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCmd tests a command retry and cancellation.
func TestCmd(t *testing.T) {
	t.Parallel()

	//low commandTimeout - fail
	cmd1 := Cmd{"sleep 2 && ls", nil, nil, "", nil, nil, 1}
	err := cmd1.RetryCommand(1)
	require.Error(t, err)

	_, err = cmd1.RetryCommandWithCombinedOutput(1)
	require.Error(t, err)

	//high commandTimeout - pass
	cmd2 := Cmd{"sleep 2 && ls", nil, nil, "", nil, nil, 3}
	err = cmd2.RetryCommand(1)
	require.NoError(t, err)

	_, err = cmd2.RetryCommandWithCombinedOutput(1)
	require.NoError(t, err)

	//no commandTimeout - pass
	cmd3 := Cmd{"sleep 2 && ls", nil, nil, "", nil, nil, 0}
	err = cmd3.RetryCommand(1)
	require.NoError(t, err)

	_, err = cmd3.RetryCommandWithCombinedOutput(1)
	require.NoError(t, err)
}

// TestSanitisedCmd checks that sanitisedCmd method only sanitises the wanted
// cases.
func TestSanitisedCmd(t *testing.T) {
	testCases := []struct {
		desc string
		cmd  Cmd
		out  string
	}{
		{
			desc: "Sanitise kubectl command",
			cmd:  Cmd{Command: "kubectl blah --kubeconfig 'not valid but needs obscuring' --more-stuff"},
			out:  "kubectl blah --kubeconfig '*****' --more-stuff",
		},
		{
			desc: "Sanitise the arg to --kubeconfig for unknown commands",
			cmd:  Cmd{Command: "idontknowthisone --kubeconfig 'the real kubeconfig is here'"},
			out:  "idontknowthisone --kubeconfig '*****'",
		},
		{
			desc: "Sanitise unknown command chain, invalid arg to --kubeconfig and URI with password",
			cmd:  Cmd{Command: "sth | piped-to-cmd https://a:notapassword@b.c --kube --none"},
			out:  "sth | piped-to-cmd https://a:*****@b.c --kube --none",
		},
		{
			desc: "Don't touch - kubectl command with invalid kubeconfig arg",
			cmd:  Cmd{Command: "kubectl stuff --kubeconfig --more-args"},
			out:  "kubectl stuff --kubeconfig --more-args",
		},
		{
			desc: "Don't touch - unknown command, invalid arg to --kubeconfig",
			cmd:  Cmd{Command: "idontknowthisone --kubeconfig --forgot-the-kubeconfig"},
			out:  "idontknowthisone --kubeconfig --forgot-the-kubeconfig",
		},
		{
			desc: "Don't touch - unknown command, invalid arg to --kubeconfig and a URI",
			cmd:  Cmd{Command: "cmd --kubeconfig --none https://a@blah.com"},
			out:  "cmd --kubeconfig --none https://a@blah.com",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()
			want := tC.out
			if got := tC.cmd.sanitisedCmd(); got != want {
				t.Errorf("Unexpected output for %q: expected %q, actual %q",
					tC.desc, want, got)
			}
		})
	}
}
