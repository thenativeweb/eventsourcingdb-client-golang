package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type writeEventsRequestBody struct {
	Preconditions *PreconditionsBody `json:"preconditions,omitempty"`
	Events        []EventCandidate   `json:"events"`
}

type WriteEventsOption func(body *writeEventsRequestBody)

func (client *Client) WriteEvents(eventCandidates []EventCandidate, preconditions ...Precondition) ([]EventContext, error) {
	requestBody := writeEventsRequestBody{
		Preconditions(preconditions...),
		eventCandidates,
	}

	if err := requestBody.Preconditions.validate(); err != nil {
		return nil, NewInvalidArgumentError("preconditions", err.Error())
	}
	if len(eventCandidates) < 1 {
		return nil, NewInvalidArgumentError("eventCandidates", "must contain at least one EventCandidate")
	}
	for i := 0; i < len(eventCandidates); i++ {
		if err := eventCandidates[i].Validate(); err != nil {
			return nil, NewInvalidArgumentError("eventCandidates", err.Error())
		}
	}

	requestBodyAsJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, NewInternalError(err)
	}

	response, err := client.requestServer(
		http.MethodPost,
		"api/v1/write-events",
		bytes.NewReader(requestBodyAsJSON),
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, NewServerError(fmt.Sprintf("failed to read the response body: %s", err.Error()))
	}

	var writeEventsResult []EventContext
	err = json.Unmarshal(responseBody, &writeEventsResult)
	if err != nil {
		return nil, NewServerError(fmt.Sprintf("failed to parse the response body: %s", err.Error()))
	}

	return writeEventsResult, nil
}
