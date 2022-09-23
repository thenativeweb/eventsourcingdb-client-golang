package retry

import (
	"math/rand"
	"time"
)

type retryError struct {
	errors []error
}

func (retryError *retryError) Error() string {
	message := "retries exceeded"

	for _, err := range retryError.errors {
		message += ": " + err.Error()
	}

	return message
}

func (retryError *retryError) AppendError(err error) {
	retryError.errors = append(retryError.errors, err)
}

func WithBackoff(fn func() error, tries int) error {
	var retryError retryError

	for retryCount := 0; retryCount < tries; retryCount++ {
		// Get a random base between 75 and 125 ms.
		base := 75 + rand.Intn(50)
		timeout := time.Duration(base*retryCount) * time.Millisecond

		// On the first iteration, retryCount is 0, so we do not wait here,
		// unless we are in an actual retry situation.
		time.Sleep(timeout)

		err := fn()
		if err != nil {
			retryError.AppendError(err)
			continue
		}

		return nil
	}

	return &retryError
}
