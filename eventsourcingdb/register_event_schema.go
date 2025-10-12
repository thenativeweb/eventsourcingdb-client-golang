package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) RegisterEventSchema(eventType string, schema map[string]any) error {
	registerEventSchemaURL, err := c.getURL("/api/v1/register-event-schema")
	if err != nil {
		return err
	}

	type RequestBody struct {
		EventType string         `json:"eventType"`
		Schema    map[string]any `json:"schema"`
	}

	requestBody := RequestBody{
		EventType: eventType,
		Schema:    schema,
	}

	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

	request := &http.Request{
		Method: http.MethodPost,
		URL:    registerEventSchemaURL,
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiToken},
			"Content-Type":  []string{"application/json"},
		},
		Body: requestBodyReader,
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = internal.ValidateServerHeader(response)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register event schema, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
	}

	return nil
}
