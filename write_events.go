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
	"net/url"
)

type writeEventsRequestBodyEventCandidateContext struct {
	Subject string  `json:"subject"`
	Type    string  `json:"type"`
	Source  url.URL `json:"source"`
}

type writeEventsRequestBodyEventCandidate struct {
	writeEventsRequestBodyEventCandidateContext
	Data any `json:"data"`
}

type writeEventsRequestBody struct {
	Preconditions *Preconditions                         `json:"preconditions,omitempty"`
	Events        []writeEventsRequestBodyEventCandidate `json:"events"`
}

func (client *Client) WriteEvents(eventCandidates []EventCandidate) ([]EventContext, error) {
	return client.WriteEventsWithPreconditions(NewPreconditions(), eventCandidates)
}

func (client *Client) WriteEventsWithPreconditions(preconditions *Preconditions, eventCandidates []EventCandidate) ([]EventContext, error) {
	requestBody := writeEventsRequestBody{
		preconditions,
		[]writeEventsRequestBodyEventCandidate{},
	}

	for _, event := range eventCandidates {
		requestBody.Events = append(requestBody.Events, writeEventsRequestBodyEventCandidate{
			writeEventsRequestBodyEventCandidateContext{
				event.Subject,
				event.Type,
				event.Source,
			},
			event.Data,
		})
	}

	requestBodyAsJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: client.configuration.timeout,
	}
	url := client.configuration.baseURL + "/api/write-events"
	request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
	if err != nil {
		return nil, err
	}

	authorization.AddAccessToken(request, client.configuration.accessToken)

	var response *http.Response
	err = retry.WithBackoff(context.Background(), client.configuration.maxTries, func() error {
		response, err = httpClient.Do(request)

		return err
	})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = client.validateProtocolVersion(response)
	if err != nil {
		return nil, err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("failed to write events: %s", responseBody))
	}

	var writeEventsResult []EventContext
	err = json.Unmarshal(responseBody, &writeEventsResult)
	if err != nil {
		return nil, err
	}

	return writeEventsResult, nil
}
