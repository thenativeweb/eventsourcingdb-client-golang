package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) WriteEvents(ctx context.Context, events []EventCandidate, preconditions []Precondition) ([]Event, error) {
	writeEventsURL, err := c.getURL("/api/v1/write-events")
	if err != nil {
		return nil, err
	}

	type RequestBodyEvent struct {
		Source      string  `json:"source"`
		Subject     string  `json:"subject"`
		Type        string  `json:"type"`
		Data        any     `json:"data"`
		TraceParent *string `json:"traceParent,omitempty"`
		TraceState  *string `json:"traceState,omitempty"`
	}

	type RequestBody struct {
		Events        []RequestBodyEvent `json:"events"`
		Preconditions []any              `json:"preconditions,omitempty"`
	}

	var requestBody RequestBody
	for _, event := range events {
		requestBody.Events = append(requestBody.Events, RequestBodyEvent(event))
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
			return nil, fmt.Errorf("unsupported predicate type: %T", precondition)
		}
	}

	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, writeEventsURL.String(), requestBodyReader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = validateServerHeader(response)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, newDBAPIError("write events", response)
	}

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
