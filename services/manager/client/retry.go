package managerclient

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/rs/zerolog"
)

// TolerateDirtyWrites defines how many dirty writes the client should tolerate before giving up
// on writing the value to the manager.
const TolerateDirtyWrites = 20

// Retry retries the passed in function up to TolerateDirtyWrites times. On first error the is not ErrVersionMismatch the retry exists.
func Retry(logger *zerolog.Logger, description string, fn func() error) error {
	var err error
	for i := range TolerateDirtyWrites {
		if i > 0 {
			wait := time.Duration(50+rand.IntN(300)) * time.Millisecond
			logger.Debug().Msgf("retry[%v/%v] %q failed due to dirty write: %v, retrying again in %s ms", i, TolerateDirtyWrites, description, err, wait)
			time.Sleep(wait)
		}

		err = fn()
		if err == nil || !errors.Is(err, ErrVersionMismatch) {
			break
		}
	}
	if err != nil {
		return err
	}
	return nil
}
