package errors

import (
	"errors"
)

// ErrServerError signals an error in the server.
// Server errors can generally not be handled by the user.
var ErrServerError = errors.New("server error")

// NewServerError returns a new server error with the given message.
func NewServerError(message string) error {
	return errors.Join(ErrServerError, errors.New(message))
}

// IsServerError returns true if the error is a server error.
//
// Deprecated: use errors.Is(err, errors.ErrServerError) instead.
func IsServerError(err error) bool {
	return errors.Is(err, ErrServerError)
}
