package retry

import (
	"context"
	"errors"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"math/rand"
	"time"
)

func getRandomizedDuration(duration int, deviation int) time.Duration {
	milliseconds := duration - deviation + rand.Intn(deviation*2)
	randomizedDuration := time.Duration(milliseconds) * time.Millisecond

	return randomizedDuration
}

func WithBackoff(ctx context.Context, tries int, fn func() error) error {
	if tries < 1 {
		return errors.New("tries must be greater than 0")
	}

	var retryError retryError

	for triesCount := 0; triesCount < tries; triesCount++ {
		// On the first iteration triesCount is 0, so the timeout is 0, and we do not wait.
		timeout := getRandomizedDuration(100, 25) * time.Duration(triesCount)

		select {
		case <-ctx.Done():
			return customErrors.NewContextCanceledError(ctx)
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
