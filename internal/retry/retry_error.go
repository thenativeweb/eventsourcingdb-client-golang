package retry

import (
	"fmt"
	"strings"
)

type RetryError struct {
	errors []error
}

func NewRetryError() *RetryError {
	retryError := &RetryError{
		errors: []error{},
	}

	return retryError
}

func (retryError *RetryError) Error() string {
	var message strings.Builder

	message.WriteString("retries exceeded")

	for retryCount, err := range retryError.errors {
		message.WriteString(fmt.Sprintf("\n\ttry %d: %s", retryCount+1, err.Error()))
	}

	return message.String()
}

func (retryError *RetryError) AppendError(err error) {
	retryError.errors = append(retryError.errors, err)
}
