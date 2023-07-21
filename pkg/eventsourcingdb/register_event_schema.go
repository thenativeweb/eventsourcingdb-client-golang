package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"io"
	"net/http"
)

type registerEventSchemaRequestBody struct {
	EventType string `json:"eventType"`
	Schema    string `json:"schema"`
}

func (client *Client) RegisterEventSchema(eventType string, JSONSchema string) error {
	if err := event.ValidateType(eventType); err != nil {
		return err
	}

	requestBody := registerEventSchemaRequestBody{
		EventType: eventType,
		Schema:    JSONSchema,
	}
	requestBodyAsJSON, err := json.Marshal(requestBody)

	routeURL := client.configuration.baseURL.JoinPath("api", "register-event-schema")
	httpClient := &http.Client{}
	request, err := http.NewRequest("POST", routeURL.String(), bytes.NewReader(requestBodyAsJSON))
	if err != nil {
		return err
	}

	authorization.AddAccessToken(request, client.configuration.accessToken)

	var response *http.Response
	var clientError error
	err = retry.WithBackoff(context.Background(), client.configuration.maxTries, func() error {
		response, err = httpClient.Do(request)

		if httputil.IsServerError(response) {
			return fmt.Errorf("server error: %s", response.Status)
		}
		if httputil.IsClientError(response) {
			var message string
			body, readBodyErr := io.ReadAll(response.Body)
			if readBodyErr != nil {
				message = string(body)
			}
			if message == "" {
				message = "unknown error"
			}

			clientError = fmt.Errorf("client error: %s: %s", response.Status, message)

			// We return nil because we don't want to retry on client error.
			return nil
		}

		return err
	})
	if err != nil {
		return err
	}
	if clientError != nil {
		return clientError
	}

	return nil
}
