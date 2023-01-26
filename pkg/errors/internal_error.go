package errors

import "fmt"

// InternalError signals an error in the internal logic of the database client.
// Internal errors can not be handled by the user and are mostly useful for debugging by the library authors.
type InternalError struct {
	cause error
}

func (err *InternalError) Error() string {
	return fmt.Sprintf("internal error: %s", err.cause.Error())
}

func NewInternalError(cause error) error {
	return &InternalError{cause}
}

func IsInternalError(err error) bool {
	_, ok := err.(*InternalError)

	return ok
}
