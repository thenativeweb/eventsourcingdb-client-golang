package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"github.com/thenativeweb/goutils/v2/coreutils/result"
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

		if err := event.ValidateSubject(subject); err != nil {
			results <- newObserveEventsError(
				customErrors.NewInvalidParameterError("subject", err.Error()),
			)
			return
		}

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

		requestFactory := httputil.NewRequestFactory(client.configuration)
		executeRequest, err := requestFactory.Create(
			http.MethodPost,
			"api/observe-events",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			results <- newObserveEventsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		response, err := executeRequest(ctx)
		if err != nil {
			results <- newObserveEventsError(err)
			return
		}
		defer response.Body.Close()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[ndjson.StreamItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				if contextutils.IsContextTerminationError(err) {
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
