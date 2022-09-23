package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
	"io"
	"net/http"
)

type writeEventsRequestBodyIsStreamPristinePrecondition struct {
	StreamName string `json:"streamName"`
}

type writeEventsRequestBodyIsStreamOnEventIDPrecondition struct {
	StreamName string `json:"streamName"`
	EventID    int    `json:"eventId"`
}

type writeEventsRequestBodyPrecondition struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type writeEventsRequestBodyEventCandidateMetadata struct {
	StreamName string `json:"streamName"`
	Name       string `json:"name"`
}

type writeEventsRequestBodyEventCandidate struct {
	Metadata writeEventsRequestBodyEventCandidateMetadata `json:"metadata"`
	Data     interface{}                                  `json:"data"`
}

type writeEventsRequestBody struct {
	Preconditions []writeEventsRequestBodyPrecondition   `json:"preconditions,omitempty"`
	Events        []writeEventsRequestBodyEventCandidate `json:"events"`
}

func (client *Client) WriteEvents(eventCandidates []EventCandidate) error {
	return client.WriteEventsWithPreconditions([]interface{}{}, eventCandidates)
}

func (client *Client) WriteEventsWithPreconditions(preconditions []interface{}, eventCandidates []EventCandidate) error {
	requestBody := writeEventsRequestBody{
		[]writeEventsRequestBodyPrecondition{},
		[]writeEventsRequestBodyEventCandidate{},
	}

	for _, precondition := range preconditions {
		switch concretePrecondition := precondition.(type) {
		case IsStreamPristinePrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, writeEventsRequestBodyPrecondition{
				"isStreamPristine",
				writeEventsRequestBodyIsStreamPristinePrecondition{concretePrecondition.StreamName},
			})
		case IsStreamOnEventIDPrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, writeEventsRequestBodyPrecondition{
				"isStreamOnEventId",
				writeEventsRequestBodyIsStreamOnEventIDPrecondition{
					concretePrecondition.StreamName,
					concretePrecondition.EventID,
				},
			})
		default:
			return errors.New(fmt.Sprintf("unknown precondition type '%+v'", precondition))
		}
	}

	for _, event := range eventCandidates {
		requestBody.Events = append(requestBody.Events, writeEventsRequestBodyEventCandidate{
			writeEventsRequestBodyEventCandidateMetadata{
				event.Metadata.StreamName,
				event.Metadata.Name,
			},
			event.Data,
		})
	}

	requestBodyAsJson, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Timeout: client.configuration.timeout,
	}
	url := client.configuration.baseUrl + "/api/write-events"
	request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJson))
	if err != nil {
		return err
	}

	var response *http.Response
	err = retry.WithBackoff(func() error {
		response, err = httpClient.Do(request)

		return err
	}, client.configuration.maxTries, context.Background())
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = client.validateProtocolVersion(response)
	if err != nil {
		return err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("failed to write events: %s", responseBody))
	}

	return nil
}
