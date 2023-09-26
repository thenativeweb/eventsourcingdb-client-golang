package errors

import (
	"errors"
	"fmt"
)

// ErrInvalidParameter signal that a parameter passed to a function is invalid.
// It is a kind of client error.
// Invalid parameter errors can generally be handled by the user.
var ErrInvalidParameter = errors.New("invalid parameter")

// NewInvalidParameterError returns a new invalid parameter error that indicates that the given parameter is invalid.
func NewInvalidParameterError(parameterName, reason string) error {
	return errors.Join(ErrClientError, ErrInvalidParameter, fmt.Errorf("parameter '%s' is invalid\n%s", parameterName, reason))
}

// IsInvalidParameterError returns true if the error is an invalid parameter error.
//
// Deprecated: use errors.Is(err, errors.ErrInvalidParameter) instead.
func IsInvalidParameterError(err error) bool {
	return errors.Is(err, ErrInvalidParameter)
}
