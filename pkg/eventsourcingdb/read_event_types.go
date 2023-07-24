package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/result"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"net/http"
)

type readEventTypesResponseItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type EventType struct {
	EventType string `json:"eventType"`
	IsPhantom bool   `json:"isPhantom"`
	Schema    string `json:"schema,omitempty"`
}

type ReadEventTypesResult struct {
	result.Result[EventType]
}

func newReadEventTypesError(err error) ReadEventTypesResult {
	return ReadEventTypesResult{
		result.NewResultWithError[EventType](err),
	}
}

func newEventType(data EventType) ReadEventTypesResult {
	return ReadEventTypesResult{
		result.NewResultWithData(data),
	}
}

func (client *Client) ReadEventTypes(ctx context.Context) <-chan ReadEventTypesResult {
	results := make(chan ReadEventTypesResult)

	go func() {
		defer close(results)

		routeURL := client.configuration.baseURL.JoinPath("api", "read-event-types")
		httpClient := &http.Client{}
		request, err := http.NewRequest("POST", routeURL.String(), bytes.NewReader(nil))
		if err != nil {
			results <- newReadEventTypesError(
				customErrors.NewInternalError(err),
			)
			return
		}
		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response
		err = retry.WithBackoff(ctx, client.configuration.maxTries, func() error {
			response, err = httpClient.Do(request)

			if httputil.IsServerError(response) {
				return fmt.Errorf("server error: %s", response.Status)
			}

			return err
		})
		if err != nil {
			if customErrors.IsContextCanceledError(err) {
				results <- newReadEventTypesError(err)
				return
			}

			results <- newReadEventTypesError(
				customErrors.NewServerError(err.Error()),
			)
			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			results <- newReadEventTypesError(
				customErrors.NewClientError(err.Error()),
			)
			return
		}

		if httputil.IsClientError(response) {
			results <- newReadEventTypesError(
				customErrors.NewClientError(response.Status),
			)
			return
		}
		if response.StatusCode != http.StatusOK {
			results <- newReadEventTypesError(
				customErrors.NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status)),
			)
			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readEventTypesResponseItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				if customErrors.IsContextCanceledError(err) {
					results <- newReadEventTypesError(err)
					return
				}

				results <- newReadEventTypesError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
				)
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				if err := json.Unmarshal(data.Payload, &serverError); err != nil {
					results <- newReadEventTypesError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
					)
					return
				}

				results <- newReadEventTypesError(customErrors.NewServerError(serverError.Error))
			case "eventType":
				var eventType EventType
				if err := json.Unmarshal(data.Payload, &eventType); err != nil {
					results <- newReadEventTypesError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
					)
					return
				}

				results <- newEventType(eventType)
			default:
				results <- newReadEventTypesError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
				)
				return
			}
		}
	}()

	return results
}
