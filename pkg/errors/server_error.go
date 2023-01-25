package errors

import "fmt"

type ServerError struct {
	message string
}

func (err *ServerError) Error() string {
	return fmt.Sprintf("server error: %s", err.message)
}

func NewServerError(message string) error {
	return &ServerError{message}
}

func IsServerError(err error) bool {
	_, ok := err.(*ServerError)

	return ok
}
