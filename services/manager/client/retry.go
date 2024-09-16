package managerclient

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/rs/zerolog"
)

// TolerateDirtyWrites defines how many dirty writes the client should tolerate before giving up
// on writing the value to the manager.
const TolerateDirtyWrites = 20

// ErrRetriesExhausted is returned when all the TolerateDirtyWrites retries fail due to a dirty write.
var ErrRetriesExhausted = errors.New("exhausted all retries")

// Retry retries the passed in function up to TolerateDirtyWrites times. On first error that is not ErrVersionMismatch
// or a total of TolerateDirtyWrites retries are executed, the underlying non-retryable error is returned or the ErrRetriesExhausted
// error is returned.
func Retry(logger *zerolog.Logger, description string, fn func() error) error {
	var err error
	var retries int

	for i := range TolerateDirtyWrites {
		retries++
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

	if retries == TolerateDirtyWrites {
		err = fmt.Errorf("%w: %w", err, ErrRetriesExhausted)
	}

	if err != nil {
		return err
	}

	return nil
}
