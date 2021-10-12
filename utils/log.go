package utils

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

const defaultLogLevel = zerolog.InfoLevel

var (
	initLogLock sync.Mutex
)

// Initialize the logging framework.
// Inputs are the golang module name used as a logging prefix
// and the env variable with the logging level
func InitLog(moduleName string, logEnvVar string) zerolog.Logger {
	initLogLock.Lock()
	defer initLogLock.Unlock()
	// set log level from env variable
	logLevel, err := getLogLevelFromEnv(logEnvVar)
	var baseLogger = zerolog.New(os.Stderr)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// create sub logger
	logger := baseLogger.With().Str("module", moduleName).Caller().Logger().Level(logLevel)
	if err != nil {
		logger.Err(err)
	} else {
		logger.Info().Msgf("Using log level %v from %s", logLevel, logEnvVar)
	}
	return logger
}

func getLogLevelFromEnv(envVarName string) (zerolog.Level, error) {
	logLevelStr, ok := os.LookupEnv(envVarName)
	var logLevel zerolog.Level
	var e error = nil
	if !ok {
		e = fmt.Errorf("%s env variable not set. Using default log level %v", envVarName, defaultLogLevel)
		logLevel = defaultLogLevel
	} else {
		level, err := convertLogLevelStr(logLevelStr)
		if err != nil {
			e = fmt.Errorf("Unsupported %s value %s. Assuming log level %v", envVarName, logLevelStr, defaultLogLevel)
			logLevel = defaultLogLevel
		} else {
			logLevel = level
		}
	}
	return logLevel, e
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
		return defaultLogLevel, fmt.Errorf("Unsupported log level %s", logLevelStr)
	} else {
		return res, nil
	}
}
