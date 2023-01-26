package errors

import "fmt"

type ClientError struct {
	message string
}

func (err *ClientError) Error() string {
	return fmt.Sprintf("client error: %s", err.message)
}

func NewClientError(message string) error {
	return &ClientError{message}
}

func IsClientError(err error) bool {
	_, ok := err.(*ClientError)

	return ok
}
