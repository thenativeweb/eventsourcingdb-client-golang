package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

func (c *Client) RegisterEventSchema(ctx context.Context, eventType string, schema map[string]any) error {
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

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, registerEventSchemaURL.String(), requestBodyReader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = validateServerHeader(response)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newDBAPIError("register event schema", response)
	}

	return nil
}
