package utils

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
)

const defaultLogLevel = zerolog.InfoLevel

var (
	isLogInit = false
	logger    zerolog.Logger
)

// Initialize the logging framework.
// Inputs are the golang module name used as a logging prefix
// and the env variable with the logging level
func InitLog(moduleName string) {
	if !isLogInit {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		// set log level from env variable
		logLevel, err := getLogLevelFromEnv()
		baseLogger := zerolog.New(os.Stderr)
		// create sub logger
		logger = baseLogger.With().Str("module", moduleName).Logger()                 // Add module name to log
		logger = logger.Level(logLevel).Output(zerolog.ConsoleWriter{Out: os.Stderr}) // Prettify the output
		logger = logger.With().Timestamp().Logger()                                   // Add time stamp
		if logLevel == zerolog.DebugLevel {
			logger = logger.With().Caller().Logger() // Add caller (line number where log message was called)
		}
		if err != nil {
			logger.Err(err)
		} else {
			logger.Info().Msgf("Using log with the level \"%v\"", logLevel)
		}
		isLogInit = true
	}
	log.Logger = logger
}

// CreateLoggerWithProjectAndClusterName creates a new logger aware of the project-name and cluster-name
// Returns the new logger
func CreateLoggerWithProjectAndClusterName(projectName, clusterName string) zerolog.Logger {
	if projectName == "" || clusterName == "" {
		return log.Logger
	}

	return log.With().
		Str("project", projectName).Str("cluster", clusterName).
		Logger()
}

func getLogLevelFromEnv() (zerolog.Level, error) {
	logLevelStr := envs.LogLevel
	level, err := convertLogLevelStr(logLevelStr)
	if err != nil {
		return defaultLogLevel, fmt.Errorf("unsupported value \"%s\" for log level. Using log level \"%v\"", logLevelStr, defaultLogLevel)
	}
	return level, err
}

func convertLogLevelStr(logLevelStr string) (zerolog.Level, error) {
	levels := map[string]zerolog.Level{
		"disabled": zerolog.Disabled,
		"panic":    zerolog.PanicLevel,
		"fatal":    zerolog.FatalLevel,
		"error":    zerolog.ErrorLevel,
		"warn":     zerolog.WarnLevel,
		"info":     zerolog.InfoLevel,
		"debug":    zerolog.DebugLevel,
		"trace":    zerolog.TraceLevel,
	}
	res, ok := levels[strings.ToLower(logLevelStr)]
	if !ok {
		return defaultLogLevel, fmt.Errorf("unsupported log level %s", logLevelStr)
	}
	return res, nil
}

// SanitiseURI replaces passwords with '*****' in connection strings that are
// in the form of <scheme>://<username>:<password>@<domain>.<tld> or
// <scheme>://<username>:<password>@<pqdn>.
func SanitiseURI(s string) string {
	// Match the password part (between ':' and '@)' of the connection string.
	cred := regexp.MustCompile(":(.*?):(.*?)@")

	// The scheme substring ({http,mongodb}://) is the first match ($1) of ':'
	// and the remaining characters are placed back to the sanitised string,
	// since that's not the credential.
	// The remaining regex delimiters ':' and '@' are then respectively
	// prepended and appended to the second match (the credential, or rather,
	// its replacement '*****').
	return cred.ReplaceAllString(s, ":$1:*****@")
}

// SanitiseKubeconfig replaces the entire kubeconfig found after the
// '--kubeconfig' flag with '*****'. This has been decided to be the superior
// option when compared to matching sensitive fields and obscuring just those.
func SanitiseKubeconfig(s string) string {
	// (?s) enables matching through newline whitespace (lf,crlf..), which is
	// relevant because the kubeconfig is likely multi-line.
	// This regex matches both single quotes ('')-delimited and double quotes
	// ("")-delimited kubeconfig, as well as a kubeconfig passed in the
	// following form: `--kubeconfig <(echo 'blah blah')`.
	kubeconfig := regexp.MustCompile(`(?s)--kubeconfig ('(.*?)'|(\"(.*?)\")|<\(echo '.*?'\))`)

	// The entire kubeconfig passed in after the flag is replaced with stars
	// and returned.
	return kubeconfig.ReplaceAllLiteralString(s, "--kubeconfig '*****'")
}
