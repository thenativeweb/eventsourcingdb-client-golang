package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) ReadEventTypes(
	ctx context.Context,
) iter.Seq2[EventType, error] {
	return func(yield func(EventType, error) bool) {
		readEventTypesURL, err := c.getURL("/api/v1/read-event-types")
		if err != nil {
			yield(EventType{}, err)
			return
		}

		type RequestBody struct{}
		requestBody := RequestBody{}

		requestBodyJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield(EventType{}, err)
			return
		}

		requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

		request := &http.Request{
			Method: http.MethodPost,
			URL:    readEventTypesURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			yield(EventType{}, err)
			return
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			yield(EventType{}, fmt.Errorf("failed to read event types, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK))
			return
		}

		for line, err := range internal.UnmarshalNDJSON(ctx, response.Body) {
			if err != nil {
				if errors.Is(err, context.Canceled) {
					// The context was canceled, which means that the
					// client is no longer interested in the events.
					// This is not an error, so we don't yield an
					// error.
					return
				}
				yield(EventType{}, err)
				return
			}

			switch line.Type {
			case "eventType":
				var streamEventType internal.StreamEventType
				err := json.Unmarshal(line.Payload, &streamEventType)
				if err != nil {
					yield(EventType{}, err)
					return
				}

				eventType := EventType{
					EventType: streamEventType.EventType,
					IsPhantom: streamEventType.IsPhantom,
					Schema:    streamEventType.Schema,
				}

				yield(eventType, nil)
				continue
			case "error":
				var error internal.Error
				err := json.Unmarshal(line.Payload, &error)
				if err != nil {
					yield(EventType{}, err)
					return
				}

				yield(EventType{}, fmt.Errorf("failed to read subjects, got error: %s", error.Error))
				return
			default:
				yield(EventType{}, fmt.Errorf("failed to handle unsupported line type: %s", line.Type))
				return
			}
		}
	}
}
