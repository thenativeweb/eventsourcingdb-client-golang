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

func (c *Client) ReadSubjects(
	ctx context.Context,
	baseSubject string,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		readSubjectsURL, err := c.getURL("/api/v1/read-subjects")
		if err != nil {
			yield("", err)
			return
		}

		type RequestBody struct {
			BaseSubject string `json:"baseSubject"`
		}

		requestBody := RequestBody{
			BaseSubject: baseSubject,
		}

		requestBodyJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield("", err)
			return
		}

		requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

		request := &http.Request{
			Method: http.MethodPost,
			URL:    readSubjectsURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			yield("", err)
			return
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			yield("", fmt.Errorf("failed to read subjects, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK))
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
				yield("", err)
				return
			}

			switch line.Type {
			case "subject":
				var streamSubject internal.StreamSubject
				err := json.Unmarshal(line.Payload, &streamSubject)
				if err != nil {
					yield("", err)
					return
				}

				yield(streamSubject.Subject, nil)
				continue
			case "error":
				var error internal.Error
				err := json.Unmarshal(line.Payload, &error)
				if err != nil {
					yield("", err)
					return
				}

				yield("", fmt.Errorf("failed to read subjects, got error: %s", error.Error))
				return
			default:
				yield("", fmt.Errorf("failed to handle unsupported line type: %s", line.Type))
				return
			}
		}
	}
}
