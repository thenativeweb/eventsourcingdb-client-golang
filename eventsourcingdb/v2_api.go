package eventsourcingdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
)

// v2API sends the requests of a client through the v2 API. It answers every
// request with status 200, and reports an error that is about the request
// itself, rather than about how it was sent, in the body of its answer: as a
// single error item, or as an error item within a stream. Every stream that
// ends on its own ends with an end item, so a stream that breaks off before
// it is incomplete.
type v2API struct {
	client *Client

	// generation is the generation of the answer with which the client chose
	// the v2 API, so that forgetting it leaves a newer answer alone.
	generation uint64
}

// open sends a request of the v2 API, as Client.request does, and returns the
// response if its status is 200, and a DBAPIError without a code otherwise.
// The caller has to close its body.
func (a v2API) open(guard *stallGuard, action string, method string, path string, body any, authorize bool) (*http.Response, error) {
	response, err := a.client.requestOfAnyServer(guard, method, path, body, authorize)
	if err != nil {
		return nil, err
	}

	// A server that answers a path of the v2 API with 404 or 405 no longer
	// offers it, e.g. since it was rolled back to a version without it, so
	// the client forgets the API, and the next request asks again. As when
	// the client asks, that holds also if the answer does not come from
	// EventSourcingDB, e.g. from a proxy that passes on nothing but the
	// paths of the v1 API. The request itself fails as with any other status.
	if isAnswerWithoutV2API(response.StatusCode) {
		a.client.apiChoice.forget(a.generation)
	}

	err = validateServerHeader(response)
	if err != nil {
		response.Body.Close()
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, newDBAPIError(action, response)
	}

	return response, nil
}

// answer reads the single item with which the v2 API answers a request, and
// returns its data if it is of the given type, or the DBAPIError of an error
// item.
func (a v2API) answer(response *http.Response, action string, itemType string) (json.RawMessage, error) {
	var item envelope
	err := parseJSON(response.Body, &item)
	if err != nil {
		return nil, err
	}

	switch item.Type {
	case itemType:
		return item.Data, nil
	case "error":
		return nil, newV2DomainError(action, item.Data)
	default:
		return nil, errUnsupportedLineType(item.Type)
	}
}

// expectEvent checks that the event with which the v2 API answers a request
// is of the given type.
func (a v2API) expectEvent(response *http.Response, action string, eventType string) error {
	data, err := a.answer(response, action, "event")
	if err != nil {
		return err
	}

	var event struct {
		Type string `json:"type"`
	}
	err = json.Unmarshal(data, &event)
	if err != nil {
		return err
	}

	if event.Type != eventType {
		return errors.New("failed to " + action)
	}

	return nil
}

// items reads the items of a stream of the v2 API, and hands on every item
// that is neither a heartbeat nor the end of the stream. An error item ends
// the stream with a DBAPIError. A finite stream ends with its end item, and
// fails if it breaks off before that item, while a stream that never ends
// fails if it ends at all.
func (a v2API) items(guard *stallGuard, action string, body io.Reader, finite bool) iter.Seq2[envelope, error] {
	return func(yield func(envelope, error) bool) {
		for item, err := range guardedLines[envelope](guard, body) {
			if err != nil {
				yield(envelope{}, err)
				return
			}

			switch {
			case item.Type == "heartbeat":
				continue
			case item.Type == "error":
				yield(envelope{}, newV2DomainError(action, item.Data))
				return
			case item.Type == "end" && finite:
				return
			}

			if !yield(item, nil) {
				return
			}
		}

		yield(envelope{}, errStreamBrokeOff(action))
	}
}

// stream sends a request of the v2 API that the server answers with a stream,
// and hands every item of it to handle, as items reads them, until handle
// returns false, or the stream ends.
func (a v2API) stream(ctx context.Context, action string, path string, body any, finite bool, fail func(err error), handle func(item envelope) bool) {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, action, methodQuery, path, body, true)
	if err != nil {
		fail(err)
		return
	}
	defer response.Body.Close()

	for item, err := range a.items(guard, action, response.Body, finite) {
		if err != nil {
			fail(err)
			return
		}

		if !handle(item) {
			return
		}
	}
}

func (a v2API) ping(ctx context.Context) error {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, actionPing, http.MethodGet, pathOfV2Ping, nil, false)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return a.expectEvent(response, actionPing, typeOfPingReceived)
}

func (a v2API) verifyAPIToken(ctx context.Context) error {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, actionVerifyAPIToken, http.MethodPost, "/api/v2/system/verify-token", nil, true)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return a.expectEvent(response, actionVerifyAPIToken, "io.eventsourcingdb.api.token-verified")
}

func (a v2API) writeEvents(ctx context.Context, body writeEventsRequestBody) ([]Event, error) {
	guard := a.client.guard(ctx)
	defer guard.end()

	writtenEvents, err := a.sendWriteEvents(guard, body)
	if err != nil {
		return nil, failureOfWrite(guard, err, v2RefusesWrite, eventsMayHaveBeenWritten)
	}

	return writtenEvents, nil
}

func (a v2API) sendWriteEvents(guard *stallGuard, body writeEventsRequestBody) ([]Event, error) {
	response, err := a.open(guard, actionWriteEvents, http.MethodPost, "/api/v2/data/write-events", body, true)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	writtenEvents := make([]Event, 0, len(body.Events))
	for item, err := range a.items(guard, actionWriteEvents, response.Body, true) {
		if err != nil {
			return nil, err
		}

		if item.Type != "event" {
			return nil, errUnsupportedLineType(item.Type)
		}

		var writtenEvent Event
		err = json.Unmarshal(item.Data, &writtenEvent)
		if err != nil {
			return nil, err
		}

		writtenEvents = append(writtenEvents, writtenEvent)
	}

	return writtenEvents, nil
}

func (a v2API) registerEventSchema(ctx context.Context, eventType string, schema map[string]any) error {
	guard := a.client.guard(ctx)
	defer guard.end()

	err := a.sendRegisterEventSchema(guard, eventType, schema)

	return failureOfWrite(guard, err, v2RefusesWrite, eventSchemaMayHaveBeenRegistered)
}

// v2RefusesWrite tells whether a server with the v2 API refused a write with
// the given error, so that it certainly did not store it: with any error item,
// which has status 200, or with a status of 400, 401, 408, 413, 429 or 503.
// This includes store-failed, which the server reports once its store has
// failed before, as it refuses every write until it is restarted. A write
// during which storing fails gets an error without a code instead, either
// status 500 or a stream that ends without its terminal item, so its outcome
// is unknown.
func v2RefusesWrite(dbAPIError *DBAPIError) bool {
	switch dbAPIError.Status {
	case http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusRequestTimeout, http.StatusRequestEntityTooLarge, http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return true
	default:
		return false
	}
}

func (a v2API) sendRegisterEventSchema(guard *stallGuard, eventType string, schema map[string]any) error {
	type RequestBody struct {
		Type   string         `json:"type"`
		Schema map[string]any `json:"schema"`
	}

	response, err := a.open(guard, actionRegisterEventSchema, http.MethodPost, "/api/v2/data/register-event-schema", RequestBody{
		Type:   eventType,
		Schema: schema,
	}, true)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	isRegistered := false
	for item, err := range a.items(guard, actionRegisterEventSchema, response.Body, true) {
		if err != nil {
			return err
		}

		if item.Type != "event" {
			return errUnsupportedLineType(item.Type)
		}

		isRegistered = true
	}

	if !isRegistered {
		return fmt.Errorf("failed to %s, the stream ended without the event that confirms it", actionRegisterEventSchema)
	}

	return nil
}

func (a v2API) readEventType(ctx context.Context, eventType string) (EventType, error) {
	guard := a.client.guard(ctx)
	defer guard.end()

	type RequestBody struct {
		Type string `json:"type"`
	}

	response, err := a.open(guard, actionReadEventType, methodQuery, "/api/v2/data/read-event-type", RequestBody{
		Type: eventType,
	}, true)
	if err != nil {
		return EventType{}, err
	}
	defer response.Body.Close()

	data, err := a.answer(response, actionReadEventType, "eventType")
	if err != nil {
		return EventType{}, err
	}

	return eventTypeOfV2(data)
}

func (a v2API) readEvents(ctx context.Context, body readEventsRequestBody) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		fail := func(err error) { yield(Event{}, err) }

		a.stream(ctx, actionReadEvents, "/api/v2/data/read-events", body, true, fail, func(item envelope) bool {
			return yieldEventOf(item.Type, item.Data, yield)
		})
	}
}

func (a v2API) observeEvents(ctx context.Context, body observeEventsRequestBody) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		fail := func(err error) { yield(Event{}, err) }

		a.stream(ctx, actionObserveEvents, "/api/v2/data/observe-events", body, false, fail, func(item envelope) bool {
			return yieldEventOf(item.Type, item.Data, yield)
		})
	}
}

func (a v2API) runEventQLQuery(ctx context.Context, query string) iter.Seq2[json.RawMessage, error] {
	return func(yield func(json.RawMessage, error) bool) {
		fail := func(err error) { yield(nil, err) }

		type RequestBody struct {
			Query string `json:"query"`
		}

		a.stream(ctx, actionRunEventQLQuery, "/api/v2/data/run-eventql-query", RequestBody{Query: query}, true, fail, func(item envelope) bool {
			if item.Type != "row" {
				fail(errUnsupportedLineType(item.Type))
				return false
			}

			// A row wraps what the query projects into, so that its data is
			// never null.
			var row struct {
				Value json.RawMessage `json:"value"`
			}
			err := json.Unmarshal(item.Data, &row)
			if err != nil {
				fail(err)
				return false
			}

			return yield(row.Value, nil)
		})
	}
}

func (a v2API) readSubjects(ctx context.Context, subject string, options ReadSubjectsOptions) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		fail := func(err error) { yield("", err) }

		type RequestBodyOptions struct {
			Recursive bool `json:"recursive"`
		}

		type RequestBody struct {
			Subject string             `json:"subject"`
			Options RequestBodyOptions `json:"options"`
		}

		requestBody := RequestBody{
			Subject: subject,
			Options: RequestBodyOptions(options),
		}

		a.stream(ctx, actionReadSubjects, "/api/v2/data/read-subjects", requestBody, true, fail, func(item envelope) bool {
			if item.Type != "subject" {
				fail(errUnsupportedLineType(item.Type))
				return false
			}

			var streamSubject streamedSubject
			err := json.Unmarshal(item.Data, &streamSubject)
			if err != nil {
				fail(err)
				return false
			}

			return yield(streamSubject.Subject, nil)
		})
	}
}

func (a v2API) readEventTypes(ctx context.Context) iter.Seq2[EventType, error] {
	return func(yield func(EventType, error) bool) {
		fail := func(err error) { yield(EventType{}, err) }

		a.stream(ctx, actionReadEventTypes, "/api/v2/data/read-event-types", nil, true, fail, func(item envelope) bool {
			if item.Type != "eventType" {
				fail(errUnsupportedLineType(item.Type))
				return false
			}

			eventType, err := eventTypeOfV2(item.Data)
			if err != nil {
				fail(err)
				return false
			}

			return yield(eventType, nil)
		})
	}
}

// eventTypeOfV2 returns an event type as the v2 API sends it as an EventType.
// Its schema is null if none is registered.
func eventTypeOfV2(data json.RawMessage) (EventType, error) {
	var eventType struct {
		Type      string          `json:"type"`
		HasEvents bool            `json:"hasEvents"`
		Schema    *map[string]any `json:"schema"`
	}
	err := json.Unmarshal(data, &eventType)
	if err != nil {
		return EventType{}, err
	}

	return EventType{
		Type:      eventType.Type,
		HasEvents: eventType.HasEvents,
		Schema:    eventType.Schema,
	}, nil
}

// newV2DomainError reports an error item of the v2 API as a DBAPIError with
// status 200.
func newV2DomainError(action string, data json.RawMessage) error {
	var errorData struct {
		Code        string          `json:"code"`
		Description string          `json:"description"`
		Details     json.RawMessage `json:"details"`
	}
	err := json.Unmarshal(data, &errorData)
	if err != nil {
		return err
	}

	return &DBAPIError{
		Action:      action,
		Status:      http.StatusOK,
		Code:        errorData.Code,
		Description: errorData.Description,
		Details:     errorData.Details,
	}
}
