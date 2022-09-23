package network

import (
	"errors"
	"math/rand"
	"net/http"
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

func Retry(fn func() (*http.Response, error), tries int) (*http.Response, error) {
	var retryError retryError

	for retryCount := 0; retryCount < tries; retryCount++ {
		// Get a random base between 75 and 125 ms.
		base := 75 + rand.Intn(50)
		timeout := time.Duration(base*retryCount) * time.Millisecond

		// On the first iteration, retryCount is 0, so we do not wait here,
		// unless we are in an actual retry situation.
		time.Sleep(timeout)

		response, err := fn()
		if err != nil {
			retryError.AppendError(err)
			continue
		}
		if response.StatusCode == http.StatusGatewayTimeout {
			response.Body.Close()
			retryError.AppendError(errors.New(http.StatusText(http.StatusGatewayTimeout)))
			continue
		}

		return response, nil
	}

	return nil, &retryError
}
