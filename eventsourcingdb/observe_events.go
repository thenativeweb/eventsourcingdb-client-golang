package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
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

func (client *Client) ObserveEvents(
	ctx context.Context,
	subject string,
	recursive ObserveRecursivelyOption,
	options ...ObserveEventsOption,
) iter.Seq2[StoreItem, error] {
	heartbeatInterval := 1 * time.Second
	heartbeatTimeout := 3 * heartbeatInterval

	return func(yield func(StoreItem, error) bool) {
		err := validateSubject(subject)
		if err != nil {
			yield(StoreItem{}, NewInvalidArgumentError("subject", err.Error()))
			return
		}

		requestOptions := observeEventsOptions{
			Recursive: recursive(),
		}
		for _, option := range options {
			err := option.apply(&requestOptions)
			if err != nil {
				yield(StoreItem{}, NewInvalidArgumentError(option.name, err.Error()))
				return
			}
		}

		requestBody := observeEventsRequest{
			Subject: subject,
			Options: requestOptions,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield(StoreItem{}, NewInternalError(err))
			return
		}

		response, err := client.requestServer(
			http.MethodPost,
			"api/observe-events",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			yield(StoreItem{}, err)
			return
		}
		defer response.Body.Close()

		heartbeatTimer := time.NewTimer(heartbeatTimeout)
		defer heartbeatTimer.Stop()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		for data, err := range ndjson.UnmarshalStream[ndjson.StreamItem](unmarshalContext, response.Body) {
			select {
			case <-heartbeatTimer.C:
				yield(StoreItem{}, NewServerError("heartbeat timeout"))
				return

			default:
				if err != nil {
					if contextutils.IsContextTerminationError(err) {
						yield(StoreItem{}, err)
						return
					}

					yield(StoreItem{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())))
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
						yield(StoreItem{}, NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())))
						return
					}

					if !yield(StoreItem{}, NewServerError(serverError.Error)) {
						return
					}

				case "item":
					var storeItem StoreItem
					if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
						yield(StoreItem{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)))
						return
					}

					if !yield(storeItem, nil) {
						return
					}

				default:
					yield(StoreItem{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)))
					return
				}
			}
		}
	}
}
