package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thenativeweb/goutils/coreutils/result"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

type readEventsRequest struct {
	Subject string            `json:"subject,omitempty"`
	Options readEventsOptions `json:"options,omitempty"`
}

type ReadEventsResult struct {
	result.Result[StoreItem]
}

func newReadEventsError(err error) ReadEventsResult {
	return ReadEventsResult{
		result.NewResultWithError[StoreItem](err),
	}
}

func newStoreItem(item StoreItem) ReadEventsResult {
	return ReadEventsResult{
		result.NewResultWithData(item),
	}
}

func (client *Client) ReadEvents(ctx context.Context, subject string, recursive ReadRecursivelyOption, options ...ReadEventsOption) <-chan ReadEventsResult {
	results := make(chan ReadEventsResult, 1)

	go func() {
		defer close(results)

		if err := event.ValidateSubject(subject); err != nil {
			results <- newReadEventsError(
				customErrors.NewInvalidParameterError("subject", err.Error()),
			)
			return
		}

		readOptions := readEventsOptions{
			Recursive: recursive(),
		}
		for _, option := range options {
			if err := option.apply(&readOptions); err != nil {
				results <- newReadEventsError(
					customErrors.NewInvalidParameterError(option.name, err.Error()),
				)
				return
			}
		}

		requestBody := readEventsRequest{
			Subject: subject,
			Options: readOptions,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			results <- newReadEventsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		routeURL := client.configuration.baseURL.JoinPath("api", "read-events")
		httpClient := &http.Client{}
		request, err := http.NewRequest("POST", routeURL.String(), bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			results <- newReadEventsError(
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
				results <- newReadEventsError(err)
				return
			}

			results <- newReadEventsError(
				customErrors.NewServerError(err.Error()),
			)
			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			results <- newReadEventsError(
				customErrors.NewClientError(err.Error()),
			)
			return
		}

		if httputil.IsClientError(response) {
			results <- newReadEventsError(
				customErrors.NewClientError(response.Status),
			)
			return
		}
		if response.StatusCode != http.StatusOK {
			results <- newReadEventsError(
				customErrors.NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status)),
			)
			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[ndjson.StreamItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				if customErrors.IsContextCanceledError(err) {
					results <- newReadEventsError(err)
					return
				}

				results <- newReadEventsError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
				)
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				if err := json.Unmarshal(data.Payload, &serverError); err != nil {
					results <- newReadEventsError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
					)
					return
				}

				results <- newReadEventsError(customErrors.NewServerError(serverError.Error))
			case "item":
				var storeItem StoreItem
				if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
					results <- newReadEventsError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
					)
					return
				}

				results <- newStoreItem(storeItem)
			default:
				results <- newReadEventsError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
				)
				return
			}
		}
	}()

	return results
}
