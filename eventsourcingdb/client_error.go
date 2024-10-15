package eventsourcingdb

import (
	"errors"

	"github.com/thenativeweb/goutils/v2/coreutils/errorutils"
)

// ErrClientError signals an error in the client code.
// Client errors can generally be handled by the user.
var ErrClientError = errors.New("client error")

func NewClientError(message string) error {
	return errorutils.Join(ErrClientError, errors.New(message))
}
