package eventsourcingdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"time"

	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
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
	err := internal.ParseJSON(response.Body, &result)
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

	response, err := a.open(guard, actionWriteEvents, http.MethodPost, "/api/v1/write-events", body, true)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var cloudEvents []internal.CloudEvent
	err = internal.ParseJSON(response.Body, &cloudEvents)
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
		return err
	}
	defer response.Body.Close()

	return nil
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

	var eventTypeResponse internal.StreamEventType
	err = internal.ParseJSON(response.Body, &eventTypeResponse)
	if err != nil {
		return EventType{}, err
	}

	return eventTypeOfV1(eventTypeResponse), nil
}

// stream sends a request of the v1 API that the server answers with a stream,
// and hands every line of it to handle, until handle returns false, or the
// stream ends. An error line ends the stream with a DBAPIError.
func (a v1API) stream(ctx context.Context, action string, path string, body any, fail func(err error), handle func(line internal.Line) bool) {
	guard := a.client.guard(ctx)
	defer guard.end()

	response, err := a.open(guard, action, http.MethodPost, path, body, true)
	if err != nil {
		fail(err)
		return
	}
	defer response.Body.Close()

	for line, err := range guardedLines[internal.Line](guard, response.Body) {
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
}

func (a v1API) readEvents(ctx context.Context, body readEventsRequestBody) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		fail := func(err error) { yield(Event{}, err) }

		a.stream(ctx, actionReadEvents, "/api/v1/read-events", body, fail, func(line internal.Line) bool {
			return yieldEventOf(line.Type, line.Payload, yield)
		})
	}
}

func (a v1API) observeEvents(ctx context.Context, body observeEventsRequestBody) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		fail := func(err error) { yield(Event{}, err) }

		a.stream(ctx, actionObserveEvents, "/api/v1/observe-events", body, fail, func(line internal.Line) bool {
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

		a.stream(ctx, actionRunEventQLQuery, "/api/v1/run-eventql-query", RequestBody{Query: query}, fail, func(line internal.Line) bool {
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
		// reading the subject alone keeps only the subject itself.
		type RequestBody struct {
			BaseSubject string `json:"baseSubject"`
		}

		a.stream(ctx, actionReadSubjects, "/api/v1/read-subjects", RequestBody{BaseSubject: subject}, fail, func(line internal.Line) bool {
			if line.Type != "subject" {
				fail(errUnsupportedLineType(line.Type))
				return false
			}

			var streamSubject internal.StreamSubject
			err := json.Unmarshal(line.Payload, &streamSubject)
			if err != nil {
				fail(err)
				return false
			}

			if !options.Recursive {
				if streamSubject.Subject != subject {
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

		a.stream(ctx, actionReadEventTypes, "/api/v1/read-event-types", struct{}{}, fail, func(line internal.Line) bool {
			if line.Type != "eventType" {
				fail(errUnsupportedLineType(line.Type))
				return false
			}

			var streamEventType internal.StreamEventType
			err := json.Unmarshal(line.Payload, &streamEventType)
			if err != nil {
				fail(err)
				return false
			}

			return yield(eventTypeOfV1(streamEventType), nil)
		})
	}
}

// yieldEventOf hands the event that a line or an item of a stream carries to
// yield, and returns whether to go on reading. A line or item of any other
// type ends the stream with an error.
func yieldEventOf(lineType string, payload json.RawMessage, yield func(Event, error) bool) bool {
	if lineType != "event" {
		yield(Event{}, errUnsupportedLineType(lineType))
		return false
	}

	var event Event
	err := json.Unmarshal(payload, &event)
	if err != nil {
		yield(Event{}, err)
		return false
	}

	return yield(event, nil)
}

// errUnsupportedLineType reports a line or an item of a stream of a type that
// the stream does not have.
func errUnsupportedLineType(lineType string) error {
	return fmt.Errorf("failed to handle unsupported line type: %s", lineType)
}

// eventTypeOfV1 returns an event type as the v1 API sends it as an EventType,
// whose HasEvents is the opposite of IsPhantom.
func eventTypeOfV1(eventType internal.StreamEventType) EventType {
	return EventType{
		Type:      eventType.EventType,
		HasEvents: !eventType.IsPhantom,
		Schema:    eventType.Schema,
	}
}
