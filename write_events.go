package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
	"io"
	"net/http"
)

type writeEventsRequestBodyEventCandidateMetadata struct {
	StreamName string `json:"streamName"`
	Name       string `json:"name"`
}

type writeEventsRequestBodyEventCandidate struct {
	Metadata writeEventsRequestBodyEventCandidateMetadata `json:"metadata"`
	Data     interface{}                                  `json:"data"`
}

type writeEventsRequestBody struct {
	Preconditions *Preconditions                         `json:"preconditions,omitempty"`
	Events        []writeEventsRequestBodyEventCandidate `json:"events"`
}

func (client *Client) WriteEvents(eventCandidates []EventCandidate) error {
	return client.WriteEventsWithPreconditions(NewPreconditions(), eventCandidates)
}

func (client *Client) WriteEventsWithPreconditions(preconditions *Preconditions, eventCandidates []EventCandidate) error {
	requestBody := writeEventsRequestBody{
		preconditions,
		[]writeEventsRequestBodyEventCandidate{},
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

	requestBodyAsJSON, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Timeout: client.configuration.timeout,
	}
	url := client.configuration.baseURL + "/api/write-events"
	request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
	if err != nil {
		return err
	}

	authorization.AddAccessToken(request, client.configuration.accessToken)

	var response *http.Response
	err = retry.WithBackoff(context.Background(), client.configuration.maxTries, func() error {
		response, err = httpClient.Do(request)

		return err
	})
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
