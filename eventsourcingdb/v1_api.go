package eventsourcingdb

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"strings"
	"time"
)

// v1API sends the requests of a client through the v1 API, for a server that
// does not offer the v2 API.
type v1API struct {
	client *Client
}

// open sends a request of the v1 API, as Client.request does, and returns the
// response if its status is 200, and a DBAPIError otherwise. The caller has to
// close its body.
func (a v1API) open(guard *stallGuard, action string, method string, path string, body any, authorize bool) (*http.Response, error) {
	response, err := a.client.request(guard, method, path, body, authorize)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, newV1DBAPIError(action, response)
	}

	return response, nil
}

// expectEvent reads the event with which the v1 API answers a request, and
// checks that it is of the given type.
func (a v1API) expectEvent(response *http.Response, action string, eventType string) error {
	var result struct {
		Type string `json:"type"`
	}
	err := parseJSON(response.Body, &result)
	if err != nil {
		return err
	}

	if result.Type != eventType {
		return errors.New("failed to " + action)
	}

	return nil
}

func (a v1API) ping(ctx context.Context) error {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, actionPing, http.MethodGet, "/api/v1/ping", nil, false)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return a.expectEvent(response, actionPing, typeOfPingReceived)
}

func (a v1API) verifyAPIToken(ctx context.Context) error {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, actionVerifyAPIToken, http.MethodPost, "/api/v1/verify-api-token", nil, true)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return a.expectEvent(response, actionVerifyAPIToken, "io.eventsourcingdb.api.api-token-verified")
}

func (a v1API) writeEvents(ctx context.Context, body writeEventsRequestBody) ([]Event, error) {
	guard := a.client.guard(ctx)
	defer guard.end()

	writtenEvents, err := a.sendWriteEvents(guard, body)
	if err != nil {
		return nil, failureOfWrite(guard, err, v1RefusesWrite, eventsMayHaveBeenWritten)
	}

	return writtenEvents, nil
}

// v1Event is an event as the v1 API sends it, with its time as text.
type v1Event struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Time            string          `json:"time"`
	Source          string          `json:"source"`
	Subject         string          `json:"subject"`
	Type            string          `json:"type"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
	Hash            string          `json:"hash"`
	PredecessorHash string          `json:"predecessorhash"`
	TraceParent     *string         `json:"traceparent"`
	TraceState      *string         `json:"tracestate"`
	Signature       *string         `json:"signature"`
}

func (a v1API) sendWriteEvents(guard *stallGuard, body writeEventsRequestBody) ([]Event, error) {
	response, err := a.open(guard, actionWriteEvents, http.MethodPost, "/api/v1/write-events", body, true)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var cloudEvents []v1Event
	err = parseJSON(response.Body, &cloudEvents)
	if err != nil {
		return nil, err
	}

	writtenEvents := make([]Event, 0, len(cloudEvents))
	for _, cloudEvent := range cloudEvents {
		cloudEventTime, err := time.Parse(time.RFC3339Nano, cloudEvent.Time)
		if err != nil {
			return nil, err
		}

		writtenEvent := Event{
			SpecVersion:     cloudEvent.SpecVersion,
			ID:              cloudEvent.ID,
			Time:            cloudEventTime,
			Source:          cloudEvent.Source,
			Subject:         cloudEvent.Subject,
			Type:            cloudEvent.Type,
			DataContentType: cloudEvent.DataContentType,
			Data:            cloudEvent.Data,
			Hash:            cloudEvent.Hash,
			PredecessorHash: cloudEvent.PredecessorHash,
			TraceParent:     cloudEvent.TraceParent,
			TraceState:      cloudEvent.TraceState,
			Signature:       cloudEvent.Signature,
		}
		writtenEvents = append(writtenEvents, writtenEvent)
	}

	return writtenEvents, nil
}

func (a v1API) registerEventSchema(ctx context.Context, eventType string, schema map[string]any) error {
	guard := a.client.guard(ctx)
	defer guard.end()

	type RequestBody struct {
		EventType string         `json:"eventType"`
		Schema    map[string]any `json:"schema"`
	}

	response, err := a.open(guard, actionRegisterEventSchema, http.MethodPost, "/api/v1/register-event-schema", RequestBody{
		EventType: eventType,
		Schema:    schema,
	}, true)
	if err != nil {
		return failureOfWrite(guard, err, v1RefusesWrite, eventSchemaMayHaveBeenRegistered)
	}
	defer response.Body.Close()

	return nil
}

// v1RefusesWrite tells whether a server with the v1 API refused a write with
// the given error, so that it certainly did not store it: with a status of
// 4xx, 503 or 507.
func v1RefusesWrite(dbAPIError *DBAPIError) bool {
	status := dbAPIError.Status

	return (status >= 400 && status < 500) || status == http.StatusServiceUnavailable || status == http.StatusInsufficientStorage
}

func (a v1API) readEventType(ctx context.Context, eventType string) (EventType, error) {
	guard := a.client.guard(ctx)
	defer guard.end()

	type RequestBody struct {
		EventType string `json:"eventType"`
	}

	response, err := a.open(guard, actionReadEventType, http.MethodPost, "/api/v1/read-event-type", RequestBody{
		EventType: eventType,
	}, true)
	if err != nil {
		return EventType{}, err
	}
	defer response.Body.Close()

	var eventTypeResponse v1EventType
	err = parseJSON(response.Body, &eventTypeResponse)
	if err != nil {
		return EventType{}, err
	}

	return eventTypeOfV1(eventTypeResponse), nil
}

// stream sends a request of the v1 API that the server answers with a stream,
// and hands every line of it to handle, until handle returns false, or the
// stream ends. An error line ends the stream with a DBAPIError. A stream of
// the v1 API has no end line, so it ends with its last line, unless it never
// ends, in which case it fails if it ends at all.
func (a v1API) stream(ctx context.Context, action string, path string, body any, endless bool, fail func(err error), handle func(line v1Line) bool) {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, action, http.MethodPost, path, body, true)
	if err != nil {
		fail(err)
		return
	}
	defer response.Body.Close()

	for line, err := range guardedLines[v1Line](guard, response.Body) {
		if err != nil {
			fail(err)
			return
		}

		if line.Type == "error" {
			fail(newV1StreamError(action, line.Payload))
			return
		}

		if !handle(line) {
			return
		}
	}

	if endless {
		fail(errStreamBrokeOff(action))
	}
}

func (a v1API) readEvents(ctx context.Context, body readEventsRequestBody) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		fail := func(err error) { yield(Event{}, err) }

		a.stream(ctx, actionReadEvents, "/api/v1/read-events", body, false, fail, func(line v1Line) bool {
			return yieldEventOf(line.Type, line.Payload, yield)
		})
	}
}

func (a v1API) observeEvents(ctx context.Context, body observeEventsRequestBody) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		fail := func(err error) { yield(Event{}, err) }

		a.stream(ctx, actionObserveEvents, "/api/v1/observe-events", body, true, fail, func(line v1Line) bool {
			if line.Type == "heartbeat" {
				return true
			}

			return yieldEventOf(line.Type, line.Payload, yield)
		})
	}
}

func (a v1API) runEventQLQuery(ctx context.Context, query string) iter.Seq2[json.RawMessage, error] {
	return func(yield func(json.RawMessage, error) bool) {
		fail := func(err error) { yield(nil, err) }

		type RequestBody struct {
			Query string `json:"query"`
		}

		a.stream(ctx, actionRunEventQLQuery, "/api/v1/run-eventql-query", RequestBody{Query: query}, false, fail, func(line v1Line) bool {
			switch line.Type {
			case "heartbeat":
				return true
			case "row":
				return yield(line.Payload, nil)
			default:
				fail(errUnsupportedLineType(line.Type))
				return false
			}
		})
	}
}

func (a v1API) readSubjects(ctx context.Context, subject string, options ReadSubjectsOptions) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		fail := func(err error) { yield("", err) }

		// The v1 API always reads recursively, from the base subject on, so
		// reading the subject alone keeps only the subject itself, as the
		// server writes it: with a single slash at its start, and none at its
		// end, e.g. "/books/42" for "/books/42/".
		type RequestBody struct {
			BaseSubject string `json:"baseSubject"`
		}

		normalizedSubject := "/" + strings.Trim(subject, "/")

		a.stream(ctx, actionReadSubjects, "/api/v1/read-subjects", RequestBody{BaseSubject: subject}, false, fail, func(line v1Line) bool {
			if line.Type != "subject" {
				fail(errUnsupportedLineType(line.Type))
				return false
			}

			var streamSubject streamedSubject
			err := json.Unmarshal(line.Payload, &streamSubject)
			if err != nil {
				fail(err)
				return false
			}

			if !options.Recursive {
				if streamSubject.Subject != normalizedSubject {
					return true
				}

				// No other subject is the subject itself, so there is nothing
				// left to read.
				yield(streamSubject.Subject, nil)
				return false
			}

			return yield(streamSubject.Subject, nil)
		})
	}
}

func (a v1API) readEventTypes(ctx context.Context) iter.Seq2[EventType, error] {
	return func(yield func(EventType, error) bool) {
		fail := func(err error) { yield(EventType{}, err) }

		a.stream(ctx, actionReadEventTypes, "/api/v1/read-event-types", struct{}{}, false, fail, func(line v1Line) bool {
			if line.Type != "eventType" {
				fail(errUnsupportedLineType(line.Type))
				return false
			}

			var streamEventType v1EventType
			err := json.Unmarshal(line.Payload, &streamEventType)
			if err != nil {
				fail(err)
				return false
			}

			return yield(eventTypeOfV1(streamEventType), nil)
		})
	}
}

// v1EventType is an event type as the v1 API sends it.
type v1EventType struct {
	EventType string          `json:"eventType"`
	IsPhantom bool            `json:"isPhantom"`
	Schema    *map[string]any `json:"schema,omitempty"`
}

// eventTypeOfV1 returns an event type as the v1 API sends it as an EventType,
// whose HasEvents is the opposite of IsPhantom.
func eventTypeOfV1(eventType v1EventType) EventType {
	return EventType{
		Type:      eventType.EventType,
		HasEvents: !eventType.IsPhantom,
		Schema:    eventType.Schema,
	}
}
