package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"net/http"

	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
)

type registerEventSchemaRequestBody struct {
	EventType string `json:"eventType"`
	Schema    string `json:"schema"`
}

func (client *Client) RegisterEventSchema(eventType string, JSONSchema string) error {
	if err := validateEventType(eventType); err != nil {
		return customErrors.NewClientError(err.Error())
	}

	requestBody := registerEventSchemaRequestBody{
		EventType: eventType,
		Schema:    JSONSchema,
	}
	requestBodyAsJSON, err := json.Marshal(requestBody)
	if err != nil {
		return customErrors.NewInternalError(err)
	}

	requestFactory := httputil.NewRequestFactory(client.configuration)
	executeRequest, err := requestFactory.Create(
		http.MethodPost,
		"api/register-event-schema",
		bytes.NewReader(requestBodyAsJSON),
	)
	if err != nil {
		return customErrors.NewInternalError(err)
	}

	response, err := executeRequest()
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}
