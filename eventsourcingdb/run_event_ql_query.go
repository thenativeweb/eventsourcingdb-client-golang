package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) RunEventQLQuery(
	ctx context.Context,
	query string,
) iter.Seq2[json.RawMessage, error] {
	return func(yield func(json.RawMessage, error) bool) {
		runEventQLQueryURL, err := c.getURL("/api/v1/run-eventql-query")
		if err != nil {
			yield(nil, err)
			return
		}

		type RequestBody struct {
			Query string `json:"query"`
		}

		requestBody := RequestBody{
			Query: query,
		}

		requestBodyJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield(nil, err)
			return
		}

		requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

		guard := c.guard(ctx)
		defer guard.end()

		request := (&http.Request{
			Method: http.MethodPost,
			URL:    runEventQLQueryURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}).WithContext(guard.ctx)

		response, err := c.send(guard, request)
		if err != nil {
			yield(nil, err)
			return
		}
		defer response.Body.Close()

		err = validateServerHeader(response)
		if err != nil {
			yield(nil, err)
			return
		}

		if response.StatusCode != http.StatusOK {
			yield(nil, newDBAPIError("run EventQL query", response))
			return
		}

		for line, err := range guard.lines(response.Body, true) {
			if err != nil {
				yield(nil, err)
				return
			}

			switch line.Type {
			case "heartbeat":
				continue
			case "row":
				if !yield(line.Payload, nil) {
					return
				}
				continue
			case "error":
				var error internal.Error
				err := json.Unmarshal(line.Payload, &error)
				if err != nil {
					yield(nil, err)
					return
				}

				yield(nil, fmt.Errorf("failed to run EventQL query, got error: %s", error.Error))
				return
			default:
				yield(nil, fmt.Errorf("failed to handle unsupported line type: %s", line.Type))
				return
			}
		}
	}
}
