package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/util"
)

type readEventsRequest struct {
	Subject string            `json:"subject,omitempty"`
	Options readEventsOptions `json:"options,omitempty"`
}

func (client *Client) ReadEvents(ctx context.Context, subject string, recursive ReadRecursivelyOption, options ...ReadEventsOption) iter.Seq2[StoreItem, error] {
	return func(yield func(StoreItem, error) bool) {
		err := validateSubject(subject)
		if err != nil {
			yield(StoreItem{}, NewInvalidArgumentError("subject", err.Error()))
			return
		}

		readOptions := readEventsOptions{
			Recursive: recursive(),
		}
		for _, option := range options {
			err := option.apply(&readOptions)
			if err != nil {
				yield(StoreItem{}, NewInvalidArgumentError(option.name, err.Error()))
				return
			}
		}

		requestBody := readEventsRequest{
			Subject: subject,
			Options: readOptions,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield(StoreItem{}, NewInternalError(err))
			return
		}

		response, err := client.requestServer(
			http.MethodPost,
			"api/read-events",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			yield(StoreItem{}, err)
			return
		}
		defer response.Body.Close()

		for data, err := range ndjson.UnmarshalStream[ndjson.StreamItem](ctx, response.Body) {
			if err != nil {
				if util.IsContextTerminationError(err) {
					yield(StoreItem{}, err)
					return
				}

				yield(StoreItem{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())))
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				err := json.Unmarshal(data.Payload, &serverError)
				if err != nil {
					yield(StoreItem{}, NewServerError(fmt.Sprintf("unexpected stream error encountered: %s", err.Error())))
					return
				}

				if !yield(StoreItem{}, NewServerError(serverError.Error)) {
					return
				}

			case "item":
				var storeItem StoreItem
				err := json.Unmarshal(data.Payload, &storeItem)
				if err != nil {
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
