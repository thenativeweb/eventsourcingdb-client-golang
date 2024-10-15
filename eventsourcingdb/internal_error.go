package eventsourcingdb

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
