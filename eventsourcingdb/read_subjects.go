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

func (c *Client) ReadSubjects(
	ctx context.Context,
	subject string,
	options ReadSubjectsOptions,
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

		// The v1 API always reads recursively, from the base subject on, so
		// reading the subject alone keeps only the subject itself.
		requestBody := RequestBody{
			BaseSubject: subject,
		}

		requestBodyJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield("", err)
			return
		}

		requestBodyReader := io.NopCloser(bytes.NewReader(requestBodyJSON))

		guard := c.guard(ctx)
		defer guard.end()

		request := (&http.Request{
			Method: http.MethodPost,
			URL:    readSubjectsURL,
			Header: http.Header{
				"Authorization": []string{"Bearer " + c.apiToken},
				"Content-Type":  []string{"application/json"},
			},
			Body: requestBodyReader,
		}).WithContext(guard.ctx)

		response, err := c.send(guard, request)
		if err != nil {
			yield("", err)
			return
		}
		defer response.Body.Close()

		err = validateServerHeader(response)
		if err != nil {
			yield("", err)
			return
		}

		if response.StatusCode != http.StatusOK {
			yield("", newV1DBAPIError(actionReadSubjects, response))
			return
		}

		for line, err := range guard.lines(response.Body) {
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

				if !options.Recursive {
					if streamSubject.Subject != subject {
						continue
					}

					// No other subject is the subject itself, so there is
					// nothing left to read.
					yield(streamSubject.Subject, nil)
					return
				}

				if !yield(streamSubject.Subject, nil) {
					return
				}
				continue
			case "error":
				yield("", newV1StreamError(actionReadSubjects, line.Payload))
				return
			default:
				yield("", fmt.Errorf("failed to handle unsupported line type: %s", line.Type))
				return
			}
		}
	}
}
