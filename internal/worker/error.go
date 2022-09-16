package worker

import "github.com/rs/zerolog/log"

// ErrorLogger function defines a callback for handling errors
func ErrorLogger(err error) {
	log.Error().Err(err).Send()
}
