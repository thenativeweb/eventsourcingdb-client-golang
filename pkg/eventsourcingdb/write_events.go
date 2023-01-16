package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

type writeEventsRequestBody struct {
	Preconditions *PreconditionsBody `json:"preconditions,omitempty"`
	Events        []event.Candidate  `json:"events"`
}

type WriteEventsOption func(body *writeEventsRequestBody)

func (client *Client) WriteEvents(eventCandidates []event.Candidate, preconditions ...Precondition) ([]event.Context, error) {
	requestBody := writeEventsRequestBody{
		Preconditions(preconditions...),
		eventCandidates,
	}

	if err := requestBody.Preconditions.validate(); err != nil {
		return nil, err
	}
	for i := 0; i < len(eventCandidates); i++ {
		if err := eventCandidates[i].Validate(); err != nil {
			return nil, err
		}
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
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to write events, database responded with status: %s", response.Status)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var writeEventsResult []event.Context
	err = json.Unmarshal(responseBody, &writeEventsResult)
	if err != nil {
		return nil, err
	}

	return writeEventsResult, nil
}
