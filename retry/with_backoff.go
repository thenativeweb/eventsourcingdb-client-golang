package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

func WithBackoff(fn func() error, tries int, context context.Context) error {
	if tries < 1 {
		return errors.New("tries must be greater than 0")
	}

	var retryError retryError

	for retryCount := 0; retryCount < tries; retryCount++ {
		// Get a random base between 75 and 125 ms.
		base := 75 + rand.Intn(50)
		timeout := time.Duration(base*retryCount) * time.Millisecond

		// On the first iteration, retryCount is 0, so we do not wait here,
		// unless we are in an actual retry situation.

		select {
		case <-context.Done():
			return context.Err()
		case <-time.After(timeout):
			err := fn()
			if err != nil {
				retryError.AppendError(err)
				continue
			}

			return nil
		}
	}

	return &retryError
}
