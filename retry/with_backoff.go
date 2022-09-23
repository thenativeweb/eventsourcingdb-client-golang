package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

func getRandomizedDuration(duration int, deviation int) time.Duration {
	milliseconds := duration - deviation + rand.Intn(deviation*2)
	randomizedDuration := time.Duration(milliseconds) * time.Millisecond

	return randomizedDuration
}

func WithBackoff(fn func() error, tries int, context context.Context) error {
	if tries < 1 {
		return errors.New("tries must be greater than 0")
	}

	var retryError retryError

	for triesCount := 0; triesCount < tries; triesCount++ {
		// On the first iteration, triesCount is 0, so we the timeout is 0, and we do not wait.
		timeout := getRandomizedDuration(100, 25) * time.Duration(triesCount)

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
