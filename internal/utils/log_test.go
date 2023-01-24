package utils

import "testing"

// TestSanitiseURI tests the SanitiseURI function.
func TestSanitiseURI(t *testing.T) {
	testCases := []struct {
		desc string
		in   string
		out  string
	}{
		{
			desc: "Strip password from log message",
			in:   "mongodb://USERNAME:secretPassword@domain.tld/",
			out:  "mongodb://USERNAME:*****@domain.tld/",
		},
		{
			desc: "Strip password from short PQDN",
			in:   "https://uname:secret@pqdn",
			out:  "https://uname:*****@pqdn",
		},
		{
			desc: "Don't sanitise URI without password",
			in:   "mailto://nobody@domain.tld",
			out:  "mailto://nobody@domain.tld",
		},
		{
			desc: "Don't sanitise hostname",
			in:   "http://somehostname:port",
			out:  "http://somehostname:port",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			want := tC.out

			if got := SanitiseURI(tC.in); got != want {
				t.Errorf("Unexpected output for %q: expected %q, actual %q",
					tC.desc, want, got)
			}
		})
	}
}
