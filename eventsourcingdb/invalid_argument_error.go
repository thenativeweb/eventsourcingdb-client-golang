package eventsourcingdb

import (
	"errors"
	"fmt"
)

// ErrInvalidArgument signals that an argument passed to a function is invalid.
// It is a kind of client error.
// Invalid argument errors can generally be handled by the user.
var ErrInvalidArgument = errors.New("invalid argument")

// NewInvalidArgumentError returns a new invalid argument error that indicates that the given argument is invalid.
func NewInvalidArgumentError(parameterName, reason string) error {
	return errors.Join(ErrClientError, ErrInvalidArgument, fmt.Errorf("argument '%s' is invalid: %s", parameterName, reason))
}
