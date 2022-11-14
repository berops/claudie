package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/Berops/claudie/internal/envs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		logger = baseLogger.With().Str("module", moduleName).Caller().Logger()        // add module name to log
		logger = logger.Level(logLevel).Output(zerolog.ConsoleWriter{Out: os.Stderr}) //prettify the output
		logger = logger.With().Timestamp().Logger()                                   //add time stamp
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
