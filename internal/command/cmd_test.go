package command

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetNewBackoff(t *testing.T) {
	t.Run("gradual increase", func(t *testing.T) {
		iterations := []struct {
			iteration int
			expected  time.Duration
		}{
			{0, 5 * time.Second},   // 5 * (2^0) = 5s
			{1, 10 * time.Second},  // 5 * (2^1) = 10s
			{2, 20 * time.Second},  // 5 * (2^2) = 20s
			{3, 40 * time.Second},  // 5 * (2^3) = 40s
			{4, 80 * time.Second},  // 5 * (2^4) = 80s
			{5, 160 * time.Second}, // 5 * (2^5) = 160s
			{6, 300 * time.Second}, // 5 * (2^6) = 300s
			{7, 300 * time.Second}, // 5 * (2^7) = 300s
			{8, 300 * time.Second}, // 5 * (2^8) = 300s
		}

		for _, tc := range iterations {
			t.Run(fmt.Sprintf("iteration_%d", tc.iteration), func(t *testing.T) {
				got := getNewBackoff(tc.iteration)
				if got != tc.expected {
					t.Errorf("iteration %d: expected %v, got %v", tc.iteration, tc.expected, got)
				}
			})
		}
	})

	t.Run("each iteration is larger than the previous", func(t *testing.T) {
		prev := getNewBackoff(0)
		for i := 1; i <= 5; i++ {
			curr := getNewBackoff(i)
			if curr <= prev {
				t.Errorf("iteration %d: expected backoff (%v) to be greater than previous (%v)", i, curr, prev)
			}
			prev = curr
		}
	})

	t.Run("does not exceed maxBackoff", func(t *testing.T) {
		for _, i := range []int{100, 500, 1000} {
			got := getNewBackoff(i)
			if got > maxBackoff {
				t.Errorf("iteration %d: backoff %v exceeded maxBackoff %v", i, got, maxBackoff)
			}
			if got != maxBackoff {
				t.Errorf("iteration %d: expected backoff to be capped at maxBackoff (%v), got %v", i, maxBackoff, got)
			}
		}
	})

	t.Run("caps exactly at maxBackoff boundary", func(t *testing.T) {
		// find the first iteration that should hit the cap
		var capIteration int
		for i := range 1000 {
			raw := time.Duration(5*(math.Pow(2, float64(i)))) * time.Second
			if raw >= maxBackoff {
				capIteration = i
				break
			}
		}

		// one step before the cap should still be below maxBackoff
		if capIteration > 0 {
			belowCap := getNewBackoff(capIteration - 1)
			if belowCap >= maxBackoff {
				t.Errorf("iteration %d: expected backoff (%v) to be below maxBackoff (%v)", capIteration-1, belowCap, maxBackoff)
			}
		}

		// at and beyond the cap iteration should return exactly maxBackoff
		for _, i := range []int{capIteration, capIteration + 1, capIteration + 10} {
			got := getNewBackoff(i)
			if got != maxBackoff {
				t.Errorf("iteration %d: expected maxBackoff (%v), got %v", i, maxBackoff, got)
			}
		}
	})
}

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
	cmd2 := Cmd{"sleep 2 && ls", nil, nil, "", nil, nil, 3 * time.Second}
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
