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

		request := (&http.Request{
			Method: http.MethodPost,
			URL:    readSubjectsURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}).WithContext(ctx)

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			yield("", err)
			return
		}
		defer response.Body.Close()

		err = internal.ValidateServerHeader(response)
		if err != nil {
			yield("", err)
			return
		}

		if response.StatusCode != http.StatusOK {
			yield("", internal.NewStatusError("read subjects", response))
			return
		}

		for line, err := range internal.UnmarshalNDJSON(ctx, response.Body) {
			if err != nil {
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

				if !yield(streamSubject.Subject, nil) {
					return
				}
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
