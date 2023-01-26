package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httpUtil"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"net/http"
	"net/url"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/result"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
)

type observeEventsRequest struct {
	Subject string               `json:"subject,omitempty"`
	Options observeEventsOptions `json:"options,omitempty"`
}

type ObserveEventsResult struct {
	result.Result[StoreItem]
}

func newObserveEventsError(err error) ObserveEventsResult {
	return ObserveEventsResult{
		result.NewResultWithError[StoreItem](err),
	}
}

func newObserveEventsValue(item StoreItem) ObserveEventsResult {
	return ObserveEventsResult{
		result.NewResultWithData(item),
	}
}

func (client *Client) ObserveEvents(ctx context.Context, subject string, recursive ObserveRecursivelyOption, options ...ObserveEventsOption) <-chan ObserveEventsResult {
	results := make(chan ObserveEventsResult, 1)

	go func() {
		defer close(results)
		requestOptions := observeEventsOptions{
			Recursive: recursive(),
		}
		for _, option := range options {
			if err := option.apply(&requestOptions); err != nil {
				results <- newObserveEventsError(
					customErrors.NewInvalidParameterError(option.name, err.Error()),
				)
				return
			}
		}

		requestBody := observeEventsRequest{
			Subject: subject,
			Options: requestOptions,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			results <- newObserveEventsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		routeURL := client.configuration.baseURL + "/api/observe-events"
		if _, err := url.Parse(routeURL); err != nil {
			results <- newObserveEventsError(
				customErrors.NewInvalidParameterError(
					"client.configuration.baseURL",
					err.Error(),
				),
			)
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		request, err := http.NewRequest(http.MethodPost, routeURL, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			results <- newObserveEventsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response
		err = retry.WithBackoff(ctx, client.configuration.maxTries, func() error {
			response, err = httpClient.Do(request)

			if httpUtil.IsServerError(response) {
				return fmt.Errorf("server error: %s", response.Status)
			}

			return err
		})
		if err != nil {
			if customErrors.IsContextCanceledError(err) {
				results <- newObserveEventsError(err)
				return
			}

			results <- newObserveEventsError(
				customErrors.NewServerError(err.Error()),
			)
			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			results <- newObserveEventsError(
				customErrors.NewClientError(err.Error()),
			)
			return
		}

		if httpUtil.IsClientError(response) {
			results <- newObserveEventsError(
				customErrors.NewClientError(response.Status),
			)
			return
		}
		if response.StatusCode != http.StatusOK {
			results <- newObserveEventsError(
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
					results <- newObserveEventsError(err)
					return
				}

				results <- newObserveEventsError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
				)
				return
			}

			switch data.Type {
			case "heartbeat":
				continue
			case "error":
				var serverError streamError
				if err := json.Unmarshal(data.Payload, &serverError); err != nil {
					results <- newObserveEventsError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
					)
					return
				}

				results <- newObserveEventsError(customErrors.NewServerError(serverError.Error))
			case "item":
				var storeItem StoreItem
				if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
					results <- newObserveEventsError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
					)
					return
				}

				results <- newObserveEventsValue(storeItem)
			default:
				results <- newObserveEventsError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
				)
				return
			}
		}
	}()

	return results
}
