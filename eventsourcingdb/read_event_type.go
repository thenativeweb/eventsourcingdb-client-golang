package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) ReadEventType(
	eventType string,
) (EventType, error) {
	readEventTypeURL, err := c.getURL("/api/v1/read-event-type")
	if err != nil {
		return EventType{}, err
	}

	type RequestBody struct {
		EventType string `json:"eventType"`
	}
	requestBody := RequestBody{
		EventType: eventType,
	}

	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return EventType{}, err
	}

	requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

	request := &http.Request{
		Method: http.MethodPost,
		URL:    readEventTypeURL,
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiToken},
			"Content-Type":  []string{"application/json"},
		},
		Body: requestBodyReader,
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return EventType{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return EventType{}, fmt.Errorf("failed to read event type, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
	}

	var eventTypeResponse internal.StreamEventType
	err = internal.ParseJSON(response.Body, &eventTypeResponse)
	if err != nil {
		return EventType{}, err
	}

	return EventType{
		EventType: eventTypeResponse.EventType,
		IsPhantom: eventTypeResponse.IsPhantom,
		Schema:    eventTypeResponse.Schema,
	}, nil
}
