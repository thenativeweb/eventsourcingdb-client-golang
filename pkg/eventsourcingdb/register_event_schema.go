package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

type registerEventSchemaRequestBody struct {
	EventType string `json:"eventType"`
	Schema    string `json:"schema"`
}

func (client *Client) RegisterEventSchema(eventType string, JSONSchema string) error {
	if err := event.ValidateType(eventType); err != nil {
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

	response, err := executeRequest(context.Background())
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}
