package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
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
		return nil, errors.NewInvalidParameterError("preconditions", err.Error())
	}
	if len(eventCandidates) < 1 {
		return nil, errors.NewInvalidParameterError("eventCandidates", "eventCandidates must contain at least one EventCandidate")
	}
	for i := 0; i < len(eventCandidates); i++ {
		if err := eventCandidates[i].Validate(); err != nil {
			return nil, errors.NewInvalidParameterError("eventCandidates", err.Error())
		}
	}

	requestBodyAsJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	routeURL := client.configuration.baseURL.JoinPath("api", "write-events")
	httpClient := &http.Client{}
	request, err := http.NewRequest("POST", routeURL.String(), bytes.NewReader(requestBodyAsJSON))
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	authorization.AddAccessToken(request, client.configuration.accessToken)

	var response *http.Response
	err = retry.WithBackoff(context.Background(), client.configuration.maxTries, func() error {
		response, err = httpClient.Do(request)

		if httputil.IsServerError(response) {
			return fmt.Errorf("server error: %s", response.Status)
		}

		return err
	})
	if err != nil {
		return nil, errors.NewServerError(err.Error())
	}
	defer response.Body.Close()

	err = client.validateProtocolVersion(response)
	if err != nil {
		return nil, errors.NewClientError(err.Error())
	}

	if httputil.IsClientError(response) {
		return nil, errors.NewClientError(response.Status)
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status))
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.NewServerError(fmt.Sprintf("failed to read the response body: %s", err.Error()))
	}

	var writeEventsResult []event.Context
	err = json.Unmarshal(responseBody, &writeEventsResult)
	if err != nil {
		return nil, errors.NewServerError(fmt.Sprintf("failed to parse the response body: %s", err.Error()))
	}

	return writeEventsResult, nil
}
