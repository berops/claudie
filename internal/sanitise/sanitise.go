package sanitise

import (
	"regexp"
	"strings"
)

var (
	// Match the password part (between ':' and '@)' of the connection string.
	cred = regexp.MustCompile(":(.*?):(.*?)@")

	// (?s) enables matching through newline whitespace (lf,crlf..), which is
	// relevant because the kubeconfig is likely multi-line.
	// This regex matches both single quotes ('')-delimited and double quotes
	// ("")-delimited kubeconfig, as well as a kubeconfig passed in the
	// following form: `--kubeconfig <(echo 'blah blah')`.
	kubeconfig = regexp.MustCompile(`(?s)--kubeconfig ('(.*?)'|(\"(.*?)\")|<\(echo '.*?'\))`)
)

// URI replaces passwords with '*****' in connection strings that are
// in the form of <scheme>://<username>:<password>@<domain>.<tld> or
// <scheme>://<username>:<password>@<pqdn>.
func URI(s string) string {
	// The scheme substring ({http,mongodb}://) is the first match ($1) of ':'
	// and the remaining characters are placed back to the sanitised string,
	// since that's not the credential.
	// The remaining regex delimiters ':' and '@' are then respectively
	// prepended and appended to the second match (the credential, or rather,
	// its replacement '*****').
	return cred.ReplaceAllString(s, ":$1:*****@")
}

// Kubeconfig replaces the entire kubeconfig found after the
// '--kubeconfig' flag with '*****'. This has been decided to be the superior
// option when compared to matching sensitive fields and obscuring just those.
func Kubeconfig(s string) string {
	// The entire kubeconfig passed in after the flag is replaced with stars and returned.
	return kubeconfig.ReplaceAllLiteralString(s, "--kubeconfig '*****'")
}

// String replaces all white spaces and ":" in the string to "-", and converts everything to lower case.
func String(s string) string {
	// convert to lower case
	sanitised := strings.ToLower(s)
	// replace all white space with "-"
	sanitised = strings.ReplaceAll(sanitised, " ", "-")
	// replace all ":" with "-"
	sanitised = strings.ReplaceAll(sanitised, ":", "-")
	// replace all "_" with "-"
	sanitised = strings.ReplaceAll(sanitised, "_", "-")
	return sanitised
}
