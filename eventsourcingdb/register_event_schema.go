package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type registerEventSchemaRequestBody struct {
	EventType string `json:"eventType"`
	Schema    string `json:"schema"`
}

func (client *Client) RegisterEventSchema(eventType string, JSONSchema string) error {
	if err := validateEventType(eventType); err != nil {
		return NewClientError(err.Error())
	}

	requestBody := registerEventSchemaRequestBody{
		EventType: eventType,
		Schema:    JSONSchema,
	}
	requestBodyAsJSON, err := json.Marshal(requestBody)
	if err != nil {
		return NewInternalError(err)
	}

	response, err := client.requestServer(
		http.MethodPost,
		"api/register-event-schema",
		bytes.NewReader(requestBodyAsJSON),
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}
