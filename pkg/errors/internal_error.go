package errors

import (
	"errors"
	"github.com/thenativeweb/goutils/v2/coreutils/errorutils"
)

// ErrInternalError signals an error in the internal logic of the database client.
// Internal errors can not be handled by the user and are mostly useful for debugging by the library authors.
var ErrInternalError = errors.New("internal error")

// NewInternalError returns a new internal error that wraps the given cause.
func NewInternalError(cause error) error {
	return errorutils.Join(ErrInternalError, cause)
}

// IsInternalError returns true if the error is an internal error.
//
// Deprecated: use errors.Is(err, errors.ErrInternalError) instead.
func IsInternalError(err error) bool {
	return errors.Is(err, ErrInternalError)
}
