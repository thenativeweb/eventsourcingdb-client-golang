package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

type Client struct {
	baseURL  *url.URL
	apiToken string
}

func NewClient(baseURL *url.URL, apiToken string) (*Client, error) {
	client := &Client{
		baseURL,
		apiToken,
	}

	return client, nil
}

func (c *Client) getURL(path string) (*url.URL, error) {
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	targetURL := c.baseURL.ResolveReference(urlPath)
	if err != nil {
		return nil, err
	}

	return targetURL, nil
}

func (c *Client) Ping() error {
	pingURL, err := c.getURL("/api/v1/ping")
	if err != nil {
		return err
	}

	response, err := http.Get(pingURL.String())
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to ping, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
	}

	type Result struct {
		Type string `json:"type"`
	}

	var result Result
	err = internal.ParseJSON(response.Body, &result)
	if err != nil {
		return err
	}

	if result.Type != "io.eventsourcingdb.api.ping-received" {
		return errors.New("failed to ping")
	}

	return nil
}

func (c *Client) VerifyAPIToken() error {
	verifyAPITokenURL, err := c.getURL("/api/v1/verify-api-token")
	if err != nil {
		return err
	}

	request := &http.Request{
		Method: http.MethodPost,
		URL:    verifyAPITokenURL,
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiToken},
		},
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to verify API token, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
	}

	type Result struct {
		Type string `json:"type"`
	}

	var result Result
	err = internal.ParseJSON(response.Body, &result)
	if err != nil {
		return err
	}

	if result.Type != "io.eventsourcingdb.api.api-token-verified" {
		return errors.New("failed to verify API token")
	}

	return nil
}

func (c *Client) WriteEvents(events []EventCandidate, preconditions []Precondition) ([]Event, error) {
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
		Preconditions []any              `json:"preconditions"`
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
		case isSubjectOnEventIDPrecondition:
			requestBody.Preconditions = append(requestBody.Preconditions, map[string]any{
				"type": "isSubjectOnEventId",
				"payload": map[string]any{
					"subject": precondition.Subject(),
					"eventId": precondition.EventID(),
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

	request := &http.Request{
		Method: http.MethodPost,
		URL:    writeEventsURL,
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiToken},
			"Content-Type":  []string{"application/json"},
		},
		Body: requestBodyReader,
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to write events, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
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
		}
		writtenEvents = append(writtenEvents, writtenEvent)
	}

	return writtenEvents, nil
}
