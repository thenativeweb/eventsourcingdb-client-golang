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

func (c *Client) ObserveEvents(
	ctx context.Context,
	subject string,
	options ObserveEventsOptions,
) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		observeEventsURL, err := c.getURL("/api/v1/observe-events")
		if err != nil {
			yield(Event{}, err)
			return
		}

		type RequestBodyBound struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}

		type RequestBodyFromLatestEvent struct {
			Subject          string `json:"subject"`
			Type             string `json:"type"`
			IfEventIsMissing string `json:"ifEventIsMissing"`
		}

		type RequestBodyOptions struct {
			Recursive       bool                        `json:"recursive"`
			LowerBound      *RequestBodyBound           `json:"lowerBound,omitempty"`
			FromLatestEvent *RequestBodyFromLatestEvent `json:"fromLatestEvent,omitempty"`
		}

		type RequestBody struct {
			Subject string             `json:"subject"`
			Options RequestBodyOptions `json:"options"`
		}

		requestBody := RequestBody{
			Subject: subject,
			Options: RequestBodyOptions{
				Recursive: options.Recursive,
			},
		}
		if options.LowerBound != nil {
			requestBody.Options.LowerBound = &RequestBodyBound{
				ID:   options.LowerBound.ID,
				Type: string(options.LowerBound.Type),
			}
		}
		if options.FromLatestEvent != nil {
			requestBody.Options.FromLatestEvent = &RequestBodyFromLatestEvent{
				Subject:          options.FromLatestEvent.Subject,
				Type:             string(options.FromLatestEvent.Type),
				IfEventIsMissing: string(options.FromLatestEvent.IfEventIsMissing),
			}
		}

		requestBodyJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield(Event{}, err)
			return
		}

		requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

		request := &http.Request{
			Method: http.MethodPost,
			URL:    observeEventsURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			yield(Event{}, err)
			return
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			yield(Event{}, fmt.Errorf("failed to observe events, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK))
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
				yield(Event{}, err)
				return
			}

			switch line.Type {
			case "heartbeat":
				continue
			case "event":
				var event Event
				err := json.Unmarshal(line.Payload, &event)
				if err != nil {
					yield(Event{}, err)
					return
				}

				yield(event, nil)
				continue
			case "error":
				var error internal.Error
				err := json.Unmarshal(line.Payload, &error)
				if err != nil {
					yield(Event{}, err)
					return
				}

				yield(Event{}, fmt.Errorf("failed to observe events, got error: %s", error.Error))
				return
			default:
				yield(Event{}, fmt.Errorf("failed to handle unsupported line type: %s", line.Type))
				return
			}
		}
	}
}
