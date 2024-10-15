package eventsourcingdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"
	"github.com/thenativeweb/goutils/v2/coreutils/result"
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

		response, err := client.requestServer(
			http.MethodPost, "api/read-event-types", http.NoBody,
		)
		if err != nil {
			results <- newReadEventTypesError(err)
			return
		}
		defer response.Body.Close()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readEventTypesResponseItem](unmarshalContext, response.Body)
		for data, err := range unmarshalResults {
			if err != nil {
				if contextutils.IsContextTerminationError(err) {
					results <- newReadEventTypesError(err)
					return
				}

				results <- newReadEventTypesError(
					NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
				)
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				if err := json.Unmarshal(data.Payload, &serverError); err != nil {
					results <- newReadEventTypesError(
						NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
					)
					return
				}

				results <- newReadEventTypesError(NewServerError(serverError.Error))
			case "eventType":
				var eventType EventType
				if err := json.Unmarshal(data.Payload, &eventType); err != nil {
					results <- newReadEventTypesError(
						NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
					)
					return
				}

				results <- newEventType(eventType)
			default:
				results <- newReadEventTypesError(
					NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
				)
				return
			}
		}
	}()

	return results
}
