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

	api, err := c.chooseAPI(ctx, actionWriteEvents)
	if err != nil {
		return nil, err
	}

	return api.writeEvents(ctx, body)
}

// writeEventsRequestBody is the body of a request that writes events, which
// both APIs take alike.
type writeEventsRequestBody struct {
	Events        []writeEventsRequestBodyEvent `json:"events"`
	Preconditions []any                         `json:"preconditions,omitempty"`
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
			requestBody.Preconditions = append(requestBody.Preconditions, map[string]any{
				"type": "isSubjectPristine",
				"payload": map[string]any{
					"subject": precondition.Subject(),
				},
			})
		case isSubjectPopulatedPrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, map[string]any{
				"type": "isSubjectPopulated",
				"payload": map[string]any{
					"subject": precondition.Subject(),
				},
			})
		case isSubjectOnEventIDPrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, map[string]any{
				"type": "isSubjectOnEventId",
				"payload": map[string]any{
					"subject": precondition.Subject(),
					"eventId": precondition.EventID(),
				},
			})
		case isEventQLQueryTruePrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, map[string]any{
				"type": "isEventQlQueryTrue",
				"payload": map[string]any{
					"query": precondition.Query(),
				},
			})
		default:
			return writeEventsRequestBody{}, fmt.Errorf("unsupported predicate type: %T", precondition)
		}
	}

	return requestBody, nil
}
