package eventsourcingdb

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
