package eventsourcingdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxDescriptionLength caps how much of the response body ends up in an error,
// so that an unexpected page, e.g. one from a proxy, does not flood the
// message.
const maxDescriptionLength = 4096

// DBAPIError reports an error that the server reported, so that callers can
// tell failures apart by Code and Status instead of parsing the error message.
// Every function of the client that sends a request returns it in that case,
// whether the server answers with an HTTP status other than 200, e.g. 401 for
// a token it does not accept, or with status 200 and an error in the body of
// its answer, e.g. a precondition that does not hold.
//
// Use errors.As to get it:
//
//	_, err := client.WriteEvents(ctx, events, preconditions)
//
//	var dbAPIError *eventsourcingdb.DBAPIError
//	if errors.As(err, &dbAPIError) && dbAPIError.Code == eventsourcingdb.CodePreconditionViolated {
//		// dbAPIError.Description tells why.
//	}
type DBAPIError struct {
	// Action names what the client tried to do, e.g. "write events".
	Action string

	// Status is the HTTP status the server answered with. It is 200 for an
	// error that the server reports in the body of its answer, e.g. within a
	// stream.
	Status int

	// Code identifies the error with the URL of the page that documents it,
	// exactly as the server sends it, e.g. CodePreconditionViolated. The
	// constants that start with Code name every code the client knows. It is
	// empty for an error without a code, e.g. a token that the server does not
	// accept.
	Code string

	// Description is the description the server gave, e.g. "/preconditions/0:
	// precondition isSubjectPristine does not hold", without leading and
	// trailing white space, and cut to at most 4096 bytes if the server gave
	// it as the body of its answer. It is empty if the server gave none, or if
	// reading it failed.
	Description string

	// Details locates the error and names the value at fault, as the JSON
	// object that the server sends with a code, e.g. {"precondition":0}. It is
	// nil if the server sends none.
	Details json.RawMessage

	// RetryAfter is how long the server asks to wait before sending the
	// request again, as the Retry-After header of an answer with status 429
	// says. It is zero for every other status, and if the header is missing.
	// The client never sends a request again on its own.
	RetryAfter time.Duration

	// Err is the error that kept the client from reading the description, or
	// nil.
	Err error
}

// Error returns "failed to <action>, got HTTP status code '<status>', expected
// '200'" for an error with a status other than 200, and "failed to <action>,
// got error" for one with status 200, followed by ": <description>" if the
// server gave one, or by ", and failed to read the description: <error>" if
// reading it failed.
func (e *DBAPIError) Error() string {
	message := fmt.Sprintf("failed to %s, got error", e.Action)
	if e.Status != http.StatusOK {
		message = fmt.Sprintf(
			"failed to %s, got HTTP status code '%d', expected '%d'",
			e.Action, e.Status, http.StatusOK,
		)
	}

	if e.Err != nil {
		return message + ", and failed to read the description: " + e.Err.Error()
	}

	if e.Description == "" {
		return message
	}

	return message + ": " + e.Description
}

// Unwrap returns the error that kept the client from reading the description,
// or nil, so that errors.Is and errors.As find it.
func (e *DBAPIError) Unwrap() error {
	return e.Err
}

// newDBAPIError reports that the server answered with a status other than
// 200, and reads the description the server gives from the response body. For
// status 429, it reads how long to wait from the Retry-After header.
func newDBAPIError(action string, response *http.Response) *DBAPIError {
	dbAPIError := &DBAPIError{
		Action: action,
		Status: response.StatusCode,
	}

	if response.StatusCode == http.StatusTooManyRequests {
		dbAPIError.RetryAfter = retryAfterOf(response)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxDescriptionLength))
	if err != nil {
		dbAPIError.Err = err
		return dbAPIError
	}

	dbAPIError.Description = strings.TrimSpace(string(body))

	return dbAPIError
}

// retryAfterOf returns how long the Retry-After header of a response asks to
// wait, in whole seconds, or zero if it is missing or names no number of
// seconds.
func retryAfterOf(response *http.Response) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(response.Header.Get("Retry-After")))
	if err != nil || seconds < 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
