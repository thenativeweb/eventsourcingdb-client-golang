package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"github.com/thenativeweb/goutils/v2/coreutils/retry"
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

	routeURL := client.configuration.baseURL.JoinPath("api", "register-event-schema")
	httpClient := &http.Client{}
	request, err := http.NewRequest("POST", routeURL.String(), bytes.NewReader(requestBodyAsJSON))
	if err != nil {
		return customErrors.NewInternalError(err)
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
			if readBodyErr == nil {
				message = string(body)
			}
			if message == "" {
				message = "unknown error"
			}

			clientError = fmt.Errorf(message)

			// We return nil because we don't want to retry on client error.
			return nil
		}

		return err
	})
	if err != nil {
		return customErrors.NewServerError(err.Error())
	}
	if clientError != nil {
		return customErrors.NewClientError(clientError.Error())
	}

	return nil
}
