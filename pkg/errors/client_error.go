package errors

import (
	"errors"
)

// ErrClientError signals an error in the client code.
// Client errors can generally be handled by the user.
var ErrClientError = errors.New("client error")

func NewClientError(message string) error {
	return errors.Join(ErrClientError, errors.New(message))
}

// IsClientError returns true if the error is a client error.
//
// Deprecated: use errors.Is(err, errors.ErrClientError) instead.
func IsClientError(err error) bool {
	return errors.Is(err, ErrClientError)
}
