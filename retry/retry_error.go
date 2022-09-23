package retry

type retryError struct {
	errors []error
}

func NewRetryError() *retryError {
	retryError := &retryError{
		errors: []error{},
	}

	return retryError
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
