package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
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

		requestFactory := httputil.NewRequestFactory(client.configuration)
		executeRequest, err := requestFactory.Create(
			http.MethodPost,
			"api/read-events",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			results <- newReadEventsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		response, err := executeRequest(ctx)
		if err != nil {
			results <- newReadEventsError(err)
			return
		}
		defer response.Body.Close()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[ndjson.StreamItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				if errors.Is(err, context.Canceled) {
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
