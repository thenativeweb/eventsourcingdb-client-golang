package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"
	"github.com/thenativeweb/goutils/v2/coreutils/result"
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

		if err := validateSubject(subject); err != nil {
			results <- newReadEventsError(
				NewInvalidArgumentError("subject", err.Error()),
			)
			return
		}

		readOptions := readEventsOptions{
			Recursive: recursive(),
		}
		for _, option := range options {
			if err := option.apply(&readOptions); err != nil {
				results <- newReadEventsError(
					NewInvalidArgumentError(option.name, err.Error()),
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
				NewInternalError(err),
			)
			return
		}

		response, err := client.requestServer(
			http.MethodPost,
			"api/read-events",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			results <- newReadEventsError(err)
			return
		}
		defer response.Body.Close()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[ndjson.StreamItem](unmarshalContext, response.Body)
		for data, err := range unmarshalResults {
			if err != nil {
				if contextutils.IsContextTerminationError(err) {
					results <- newReadEventsError(err)
					return
				}

				results <- newReadEventsError(
					NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
				)
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				if err := json.Unmarshal(data.Payload, &serverError); err != nil {
					results <- newReadEventsError(
						NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
					)
					return
				}

				results <- newReadEventsError(NewServerError(serverError.Error))
			case "item":
				var storeItem StoreItem
				if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
					results <- newReadEventsError(
						NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
					)
					return
				}

				results <- newStoreItem(storeItem)
			default:
				results <- newReadEventsError(
					NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
				)
				return
			}
		}
	}()

	return results
}
