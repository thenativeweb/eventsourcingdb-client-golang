package eventsourcingdb

import (
	"errors"
)

// ErrClientError signals an error in the client code.
// Client errors can generally be handled by the user.
var ErrClientError = errors.New("client error")

func NewClientError(message string) error {
	return errors.Join(ErrClientError, errors.New(message))
}
