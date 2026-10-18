package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
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

		guard := c.guard(ctx)
		defer guard.end()

		request := (&http.Request{
			Method: http.MethodPost,
			URL:    readEventTypesURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}).WithContext(guard.ctx)

		response, err := c.send(guard, request)
		if err != nil {
			yield(EventType{}, err)
			return
		}
		defer response.Body.Close()

		err = validateServerHeader(response)
		if err != nil {
			yield(EventType{}, err)
			return
		}

		if response.StatusCode != http.StatusOK {
			yield(EventType{}, newV1DBAPIError(actionReadEventTypes, response))
			return
		}

		for line, err := range guard.lines(response.Body) {
			if err != nil {
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
					Type:      streamEventType.EventType,
					HasEvents: !streamEventType.IsPhantom,
					Schema:    streamEventType.Schema,
				}

				if !yield(eventType, nil) {
					return
				}
				continue
			case "error":
				yield(EventType{}, newV1StreamError(actionReadEventTypes, line.Payload))
				return
			default:
				yield(EventType{}, fmt.Errorf("failed to handle unsupported line type: %s", line.Type))
				return
			}
		}
	}
}
