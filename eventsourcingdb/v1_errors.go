package eventsourcingdb

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

// newV1DBAPIError reports that a server with the v1 API answered with a status
// other than 200, as newDBAPIError does, and gives the error the code of the
// v2 API that it clearly matches, if there is one.
func newV1DBAPIError(action string, response *http.Response) *DBAPIError {
	dbAPIError := newDBAPIError(action, response)
	dbAPIError.Code = codeOfV1Error(action, dbAPIError.Status, dbAPIError.Description)

	return dbAPIError
}

// newV1StreamError reports an error that a server with the v1 API sent within
// a stream, as an error with status 200 and without a code.
func newV1StreamError(action string, payload json.RawMessage) error {
	var streamError internal.Error
	err := json.Unmarshal(payload, &streamError)
	if err != nil {
		return err
	}

	return &DBAPIError{
		Action:      action,
		Status:      http.StatusOK,
		Description: streamError.Error,
	}
}

// codeOfV1Error returns the code of the v2 API that a failure of the v1 API
// clearly matches, as EventSourcingDB 1.2 words it, or an empty code if there
// is none.
func codeOfV1Error(action string, status int, description string) string {
	switch {
	case action == actionWriteEvents && status == http.StatusConflict && strings.HasPrefix(description, "state conflict"):
		return CodePreconditionViolated
	case action == actionWriteEvents && status == http.StatusConflict && strings.HasPrefix(description, "schema conflict"):
		return CodeEventSchemaViolated
	case action == actionRegisterEventSchema && status == http.StatusConflict && description == "schema conflict: schema already exists":
		return CodeEventSchemaAlreadyRegistered
	case action == actionReadEventType && status == http.StatusNotFound:
		return CodeEventTypeUnknown
	case status == http.StatusServiceUnavailable && strings.HasPrefix(description, "the current operation mode conflicts with the given request"):
		return CodeStoreReadOnly
	case action == actionWriteEvents && status == http.StatusInsufficientStorage:
		return CodeLicenseInvalid
	default:
		return ""
	}
}
