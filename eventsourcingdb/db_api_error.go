package eventsourcingdb

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxReasonLength caps how much of the response body ends up in an error, so
// that an unexpected page, e.g. one from a proxy, does not flood the message.
const maxReasonLength = 4096

// DBAPIError reports that the server answered a request with an unexpected
// HTTP status code. Every function of the client that sends a request returns
// it in that case, so that callers can tell failures apart by StatusCode and
// Reason instead of parsing the error message. Different failures share a
// status code: a failed precondition and an event that does not match its
// schema, for example, are both answered with 409, and only their reasons
// differ.
//
// Use errors.As to get it:
//
//	_, err := client.WriteEvents(events, preconditions)
//
//	var dbAPIError *eventsourcingdb.DBAPIError
//	if errors.As(err, &dbAPIError) && dbAPIError.StatusCode == http.StatusConflict {
//		// dbAPIError.Reason tells why, e.g. "state conflict: precondition failed".
//	}
type DBAPIError struct {
	// Action names what the client tried to do, e.g. "write events".
	Action string

	// StatusCode is the HTTP status code the server answered with.
	StatusCode int

	// Reason is the reason the server gave in the response body, without
	// leading and trailing white space, and cut to at most 4096 bytes. It is
	// empty if the server gave none, or if reading it failed.
	Reason string

	// Err is the error that kept the client from reading the reason, or nil.
	Err error
}

// Error returns "failed to <action>, got HTTP status code '<status code>',
// expected '200'", followed by ": <reason>" if the server gave a reason, or
// by ", and failed to read the reason: <error>" if reading it failed.
func (e *DBAPIError) Error() string {
	message := fmt.Sprintf(
		"failed to %s, got HTTP status code '%d', expected '%d'",
		e.Action, e.StatusCode, http.StatusOK,
	)

	if e.Err != nil {
		return message + ", and failed to read the reason: " + e.Err.Error()
	}

	if e.Reason == "" {
		return message
	}

	return message + ": " + e.Reason
}

// Unwrap returns the error that kept the client from reading the reason, or
// nil, so that errors.Is and errors.As find it.
func (e *DBAPIError) Unwrap() error {
	return e.Err
}

// newDBAPIError reports that the server answered with an unexpected status
// code, and reads the reason the server gives from the response body.
func newDBAPIError(action string, response *http.Response) *DBAPIError {
	dbAPIError := &DBAPIError{
		Action:     action,
		StatusCode: response.StatusCode,
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxReasonLength))
	if err != nil {
		dbAPIError.Err = err
		return dbAPIError
	}

	dbAPIError.Reason = strings.TrimSpace(string(body))

	return dbAPIError
}
