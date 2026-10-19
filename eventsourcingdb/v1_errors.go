package eventsourcingdb

import (
	"encoding/json"
	"net/http"
	"strings"
)

// newV1DBAPIError reports that a server with the v1 API answered with a status
// other than 200, as newDBAPIError does, and gives the error the code of the
// v2 API that it clearly matches, if there is one.
func newV1DBAPIError(action string, response *http.Response) *DBAPIError {
	dbAPIError := newDBAPIError(action, response)
	dbAPIError.Code = codeOfV1Error(action, dbAPIError.Status, dbAPIError.Description)

	return dbAPIError
}

// v1ErrorPayload is the payload of a line with which a server with the v1 API
// reports an error within a stream.
type v1ErrorPayload struct {
	Error string `json:"error"`
}

// newV1StreamError reports an error that a server with the v1 API sent within
// a stream, as an error with status 200 and without a code.
func newV1StreamError(action string, payload json.RawMessage) error {
	var streamError v1ErrorPayload
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

// v1Code gives a failure of the v1 API the code of the v2 API that it clearly
// matches: a failure of the given action, or of any action if it is empty,
// with the given status, and with a description that starts with the given
// text, as EventSourcingDB 1.2 words it.
type v1Code struct {
	action      string
	status      int
	description string
	code        string
}

// v1Codes are the failures of the v1 API that clearly match an error of the
// v2 API. Every other one keeps an empty code.
var v1Codes = []v1Code{
	{action: actionWriteEvents, status: http.StatusConflict, description: "state conflict: precondition failed", code: CodePreconditionViolated},
	{action: actionWriteEvents, status: http.StatusConflict, description: "state conflict: exceeded max store capacity", code: CodeCapacityExceeded},
	{action: actionWriteEvents, status: http.StatusConflict, description: "schema conflict: event candidate does not match schema", code: CodeEventSchemaViolated},
	{action: actionRegisterEventSchema, status: http.StatusConflict, description: "schema conflict: schema already exists", code: CodeEventSchemaAlreadyRegistered},
	{action: actionRegisterEventSchema, status: http.StatusConflict, description: "schema conflict: event with ID", code: CodeEventSchemaViolated},
	{action: actionReadEvents, status: http.StatusConflict, description: "state conflict: fromLatestEvent results in an event ID greater than upperBound ID", code: CodeRangeUnsatisfiable},
	{action: actionReadEventType, status: http.StatusNotFound, code: CodeEventTypeUnknown},
	{status: http.StatusServiceUnavailable, description: "the current operation mode conflicts with the given request", code: CodeStoreReadOnly},
	{action: actionWriteEvents, status: http.StatusInsufficientStorage, code: CodeLicenseInvalid},
}

// codeOfV1Error returns the code of the v2 API that a failure of the v1 API
// clearly matches, or an empty code if there is none.
func codeOfV1Error(action string, status int, description string) string {
	for _, v1Code := range v1Codes {
		if (v1Code.action == "" || v1Code.action == action) && v1Code.status == status && strings.HasPrefix(description, v1Code.description) {
			return v1Code.code
		}
	}

	return ""
}
