package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/event"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
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

	requestFactory := httputil.NewRequestFactory(client.configuration)
	executeRequest, err := requestFactory.Create(
		http.MethodPost,
		"api/write-events",
		bytes.NewReader(requestBodyAsJSON),
	)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	response, err := executeRequest(context.Background())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

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
