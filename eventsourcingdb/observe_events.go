package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
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

	heartbeatInterval := 1 * time.Second
	heartbeatTimeout := heartbeatInterval * 3

	go func() {
		defer close(results)

		if err := validateSubject(subject); err != nil {
			results <- newObserveEventsError(
				NewInvalidArgumentError("subject", err.Error()),
			)
			return
		}

		requestOptions := observeEventsOptions{
			Recursive: recursive(),
		}
		for _, option := range options {
			if err := option.apply(&requestOptions); err != nil {
				results <- newObserveEventsError(
					NewInvalidArgumentError(option.name, err.Error()),
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
				NewInternalError(err),
			)
			return
		}

		response, err := client.requestServer(
			http.MethodPost,
			"api/observe-events",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			results <- newObserveEventsError(err)
			return
		}
		defer response.Body.Close()

		heartbeatTimer := time.NewTimer(heartbeatTimeout)
		defer heartbeatTimer.Stop()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[ndjson.StreamItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			select {
			case <-heartbeatTimer.C:
				results <- newObserveEventsError(
					NewServerError("heartbeat timeout"),
				)
				return

			default:
				data, err := unmarshalResult.GetData()
				if err != nil {
					if contextutils.IsContextTerminationError(err) {
						results <- newObserveEventsError(err)
						return
					}

					results <- newObserveEventsError(
						NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
					)
					return
				}

				switch data.Type {
				case "heartbeat":
					if !heartbeatTimer.Stop() {
						<-heartbeatTimer.C
					}
					heartbeatTimer.Reset(heartbeatTimeout)

				case "error":
					var serverError streamError
					if err := json.Unmarshal(data.Payload, &serverError); err != nil {
						results <- newObserveEventsError(
							NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
						)
						return
					}

					results <- newObserveEventsError(NewServerError(serverError.Error))

				case "item":
					var storeItem StoreItem
					if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
						results <- newObserveEventsError(
							NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
						)
						return
					}

					results <- newObserveEventsValue(storeItem)

				default:
					results <- newObserveEventsError(
						NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
					)
					return
				}
			}
		}
	}()

	return results
}
