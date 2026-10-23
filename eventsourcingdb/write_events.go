package eventsourcingdb

import (
	"context"
	"fmt"
)

// WriteEvents writes the given events, if all of the given preconditions
// hold, and returns them as the server wrote them.
func (c *Client) WriteEvents(ctx context.Context, events []EventCandidate, preconditions []Precondition) ([]Event, error) {
	body, err := newWriteEventsRequestBody(events, preconditions)
	if err != nil {
		return nil, err
	}

	// Only the v2 API can check a recursive precondition, so the client asks
	// the server again if it found out before that it does not offer it,
	// rather than refusing the write because of an answer that may be
	// outdated.
	chooseAPI := c.chooseAPI
	if body.hasRecursivePrecondition() {
		chooseAPI = c.chooseAPIAskingAgainIfV1
	}

	api, err := chooseAPI(ctx, actionWriteEvents)
	if err != nil {
		return nil, err
	}

	return api.writeEvents(ctx, body)
}

// writeEventsRequestBody is the body of a request that writes events, which
// both APIs take alike, but for the options of a precondition, which only the
// v2 API knows (see forV1).
type writeEventsRequestBody struct {
	Events        []writeEventsRequestBodyEvent        `json:"events"`
	Preconditions []writeEventsRequestBodyPrecondition `json:"preconditions,omitempty"`
}

type writeEventsRequestBodyPrecondition struct {
	Type    string                                    `json:"type"`
	Payload writeEventsRequestBodyPreconditionPayload `json:"payload"`
}

// writeEventsRequestBodyPreconditionPayload is the payload of a precondition,
// with the fields that its type takes, and without every other one. The
// fields are pointers, so that a field that a precondition takes is sent also
// if it is empty, while options are sent only if the caller gave them.
type writeEventsRequestBodyPreconditionPayload struct {
	Subject *string                                    `json:"subject,omitempty"`
	EventID *string                                    `json:"eventId,omitempty"`
	Query   *string                                    `json:"query,omitempty"`
	Options *writeEventsRequestBodyPreconditionOptions `json:"options,omitempty"`
}

type writeEventsRequestBodyPreconditionOptions struct {
	Recursive bool `json:"recursive"`
}

// forV1 returns the body as the v1 API takes it, which knows no options of a
// precondition. A precondition that is not recursive goes without them, since
// it means the same there, while a recursive one fails with
// ErrIncompatibleServer, since the v1 API can not check it.
func (b writeEventsRequestBody) forV1() (writeEventsRequestBody, error) {
	if b.hasRecursivePrecondition() {
		return writeEventsRequestBody{}, ErrIncompatibleServer
	}

	preconditions := make([]writeEventsRequestBodyPrecondition, 0, len(b.Preconditions))
	for _, precondition := range b.Preconditions {
		precondition.Payload.Options = nil
		preconditions = append(preconditions, precondition)
	}

	return writeEventsRequestBody{
		Events:        b.Events,
		Preconditions: preconditions,
	}, nil
}

// hasRecursivePrecondition tells whether any precondition of the body is
// recursive, which only the v2 API can check.
func (b writeEventsRequestBody) hasRecursivePrecondition() bool {
	for _, precondition := range b.Preconditions {
		options := precondition.Payload.Options
		if options != nil && options.Recursive {
			return true
		}
	}

	return false
}

type writeEventsRequestBodyEvent struct {
	Source      string  `json:"source"`
	Subject     string  `json:"subject"`
	Type        string  `json:"type"`
	Data        any     `json:"data"`
	TraceParent *string `json:"traceparent,omitempty"`
	TraceState  *string `json:"tracestate,omitempty"`
}

func newWriteEventsRequestBody(events []EventCandidate, preconditions []Precondition) (writeEventsRequestBody, error) {
	requestBody := writeEventsRequestBody{
		Events: make([]writeEventsRequestBodyEvent, 0, len(events)),
	}

	for _, event := range events {
		requestBody.Events = append(requestBody.Events, writeEventsRequestBodyEvent(event))
	}

	for _, precondition := range preconditions {
		switch precondition := precondition.(type) {
		case isSubjectPristinePrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, writeEventsRequestBodyPrecondition{
				Type: "isSubjectPristine",
				Payload: writeEventsRequestBodyPreconditionPayload{
					Subject: new(precondition.Subject()),
					Options: (*writeEventsRequestBodyPreconditionOptions)(precondition.options),
				},
			})
		case isSubjectPopulatedPrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, writeEventsRequestBodyPrecondition{
				Type: "isSubjectPopulated",
				Payload: writeEventsRequestBodyPreconditionPayload{
					Subject: new(precondition.Subject()),
					Options: (*writeEventsRequestBodyPreconditionOptions)(precondition.options),
				},
			})
		case isSubjectOnEventIDPrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, writeEventsRequestBodyPrecondition{
				Type: "isSubjectOnEventId",
				Payload: writeEventsRequestBodyPreconditionPayload{
					Subject: new(precondition.Subject()),
					EventID: new(precondition.EventID()),
					Options: (*writeEventsRequestBodyPreconditionOptions)(precondition.options),
				},
			})
		case isEventQLQueryTruePrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, writeEventsRequestBodyPrecondition{
				Type: "isEventQlQueryTrue",
				Payload: writeEventsRequestBodyPreconditionPayload{
					Query: new(precondition.Query()),
				},
			})
		default:
			return writeEventsRequestBody{}, fmt.Errorf("unsupported predicate type: %T", precondition)
		}
	}

	return requestBody, nil
}
