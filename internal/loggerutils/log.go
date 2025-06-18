package loggerutils

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
)

// WithProjectName creates a new logger aware of the project-name.
func WithProjectName(project string) zerolog.Logger {
	return logger.With().Str("project", project).Logger()
}

// WithClusterName creates a new logger aware of the cluster-name.
func WithClusterName(cluster string) zerolog.Logger {
	return logger.With().Str("cluster", cluster).Logger()
}

func WithProjectAndCluster(project, cluster string) zerolog.Logger {
	return logger.With().Str("project", project).Str("cluster", cluster).Logger()
}

func WithTaskContext(project, cluster, id string) zerolog.Logger {
	return logger.With().Str("project", project).Str("cluster", cluster).Str("task", id).Logger()
}

const defaultLogLevel = zerolog.InfoLevel

var (
	isLogInit = false
	// Available time formats https://pkg.go.dev/time#pkg-constants
	logTimeFormat = time.RFC3339 // c"2006-01-02T15:04:05Z07:00"
	logger        zerolog.Logger
)

// Initialize the logging framework.
// Inputs are the golang module name used as a logging prefix
// and the env variable with the logging level
func Init(moduleName string) {
	if !isLogInit {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		// set log level from env variable
		logLevel, err := getLogLevelFromEnv()
		baseLogger := zerolog.New(os.Stderr)
		// create sub logger
		logger = baseLogger.With().Str("module", moduleName).Logger() // Add module name to log
		logger = logger.Level(logLevel).
			Output(zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: logTimeFormat,
			}) // Prettify the output
		logger = logger.With().Timestamp().Logger() // Add time stamp
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
