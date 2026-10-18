package eventsourcingdb

import (
	"errors"
	"fmt"
)

// ErrOutcomeUnknown reports that writing events or registering an event
// schema failed in a way that leaves open whether the server stored it
// nevertheless, e.g. because the connection stalled or broke after the
// request was sent, because the context ended after that, or because the
// server failed unexpectedly. WriteEvents and RegisterEventSchema return an
// error in that case that errors.Is finds it in, and that wraps the cause, so
// that errors.Is and errors.As find the cause, too:
//
//	_, err := client.WriteEvents(ctx, events, preconditions)
//	if errors.Is(err, eventsourcingdb.ErrOutcomeUnknown) {
//		// Read the events to find out whether they were written, or write
//		// them again with a precondition that fails if they were.
//	}
//
// Any other error of WriteEvents and RegisterEventSchema means that the server
// certainly did not store anything: either the client could not connect to
// it at all, or the server refused the request.
var ErrOutcomeUnknown = errors.New("outcome unknown")

// outcomeUnknownError reports a write that may have been stored, with
// ErrOutcomeUnknown, and wraps its cause.
type outcomeUnknownError struct {
	// what says what may have been stored, e.g. "the events may have been
	// written".
	what  string
	cause error
}

func (e *outcomeUnknownError) Error() string {
	return fmt.Sprintf("%s, %s nevertheless: %s", ErrOutcomeUnknown, e.what, e.cause)
}

func (e *outcomeUnknownError) Is(target error) bool {
	return target == ErrOutcomeUnknown
}

func (e *outcomeUnknownError) Unwrap() error {
	return e.cause
}

// The texts with which outcomeUnknownError says what may have been stored.
const (
	eventsMayHaveBeenWritten         = "the events may have been written"
	eventSchemaMayHaveBeenRegistered = "the event schema may have been registered"
)

// failureOfWrite returns the error with which a write fails because of err:
// err itself if the write was certainly not stored, and an error that wraps
// it with ErrOutcomeUnknown otherwise. A write was certainly not stored only
// if the client could not connect to the server at all, or if the server
// refused it, as refused tells for an error that the server reported. The
// rule is the same in every SDK of EventSourcingDB, so it does not count a
// request that fails after connecting, but before all of it was sent, as not
// stored.
func failureOfWrite(guard *stallGuard, err error, refused func(dbAPIError *DBAPIError) bool, what string) error {
	if err == nil || !guard.isConnected() {
		return err
	}

	var dbAPIError *DBAPIError
	if errors.As(err, &dbAPIError) && refused(dbAPIError) {
		return err
	}

	return &outcomeUnknownError{what: what, cause: err}
}
