package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/util"
)

type Subject struct {
	Subject string `json:"subject"`
}

type readSubjectsRequestBody struct {
	BaseSubject string `json:"baseSubject"`
}

type readSubjectsResponseItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (client *Client) ReadSubjects(ctx context.Context, options ...ReadSubjectsOption) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		requestBody := readSubjectsRequestBody{
			BaseSubject: "/",
		}
		for _, option := range options {
			err := option.apply(&requestBody)
			if err != nil {
				yield("", NewInvalidArgumentError(option.name, err.Error()))
				return
			}
		}

		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			yield("", NewInternalError(err))
			return
		}

		response, err := client.requestServer(
			http.MethodPost,
			"api/read-subjects",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			yield("", NewInternalError(err))
			return
		}
		defer response.Body.Close()

		for data, err := range ndjson.UnmarshalStream[readSubjectsResponseItem](ctx, response.Body) {
			if err != nil {
				if util.IsContextTerminationError(err) {
					yield("", err)
					return
				}

				yield("", NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())))
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				err := json.Unmarshal(data.Payload, &serverError)
				if err != nil {
					yield("", NewServerError(fmt.Sprintf("unexpected stream error encountered: %s", err.Error())))
					return
				}

				if !yield("", NewServerError(serverError.Error)) {
					return
				}

			case "subject":
				var subject Subject
				err := json.Unmarshal(data.Payload, &subject)
				if err != nil {
					yield("", NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)))
					return
				}

				if !yield(subject.Subject, nil) {
					return
				}
			default:
				yield("", NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)))
				return
			}
		}
	}
}
