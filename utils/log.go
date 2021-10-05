package utils

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLog(moduleName string) {
	var baseLogger = zerolog.New(os.Stderr)
	var baseLevel = zerolog.InfoLevel
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(baseLevel)
	// create sub logger
	logger := baseLogger.With().Str("module", moduleName).Caller().Logger()
	log.Logger = logger
}
