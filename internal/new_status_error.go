package internal

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxReasonLength caps how much of the response body ends up in an error, so
// that an unexpected page, e.g. one from a proxy, does not flood the message.
const maxReasonLength = 4096

// NewStatusError reports that the server answered with an unexpected status
// code. It includes the reason the server gives in the response body, because
// different failures share a status code: a failed precondition and an event
// that does not match its schema, for example, are both answered with 409.
func NewStatusError(action string, response *http.Response) error {
	message := fmt.Sprintf(
		"failed to %s, got HTTP status code '%d', expected '%d'",
		action, response.StatusCode, http.StatusOK,
	)

	body, err := io.ReadAll(io.LimitReader(response.Body, maxReasonLength))
	if err != nil {
		return fmt.Errorf("%s, and failed to read the reason: %w", message, err)
	}

	reason := strings.TrimSpace(string(body))
	if reason == "" {
		return errors.New(message)
	}

	return errors.New(message + ": " + reason)
}
