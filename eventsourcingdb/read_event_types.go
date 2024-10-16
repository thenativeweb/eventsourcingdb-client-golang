package eventsourcingdb

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"
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

func (client *Client) ReadEventTypes(ctx context.Context) iter.Seq2[EventType, error] {
	return func(yield func(EventType, error) bool) {

		response, err := client.requestServer(
			http.MethodPost, "api/read-event-types", http.NoBody,
		)
		if err != nil {
			yield(EventType{}, err)
			return
		}
		defer response.Body.Close()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readEventTypesResponseItem](unmarshalContext, response.Body)
		for data, err := range unmarshalResults {
			if err != nil {
				if contextutils.IsContextTerminationError(err) {
					yield(EventType{}, err)
					return
				}

				yield(EventType{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())))
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				err := json.Unmarshal(data.Payload, &serverError)
				if err != nil {
					yield(EventType{}, NewServerError(fmt.Sprintf("unexpected stream error encountered: %s", err.Error())))
					return
				}

				if !yield(EventType{}, NewServerError(serverError.Error)) {
					return
				}
			case "eventType":
				var eventType EventType
				if err := json.Unmarshal(data.Payload, &eventType); err != nil {
					yield(EventType{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)))
					return
				}

				if !yield(eventType, nil) {
					return
				}
			default:
				yield(EventType{}, NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)))
				return
			}
		}
	}
}
