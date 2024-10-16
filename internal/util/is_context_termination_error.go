package util

import (
	"context"
	"errors"
)

// IsContextTerminationError returns true if the given error is either context.Canceled or context.DeadlineExceeded.
func IsContextTerminationError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
