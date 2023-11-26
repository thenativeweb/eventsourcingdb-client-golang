package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httputil"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/goutils/v2/coreutils/contextutils"
	"github.com/thenativeweb/goutils/v2/coreutils/result"
)

type Subject struct {
	Subject string `json:"subject"`
}

type ReadSubjectsResult struct {
	result.Result[string]
}

func newReadSubjectsError(err error) ReadSubjectsResult {
	return ReadSubjectsResult{
		result.NewResultWithError[string](err),
	}
}

func newSubject(subject Subject) ReadSubjectsResult {
	return ReadSubjectsResult{
		result.NewResultWithData(subject.Subject),
	}
}

type readSubjectsRequestBody struct {
	BaseSubject string `json:"baseSubject"`
}

type readSubjectsResponseItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (client *Client) ReadSubjects(ctx context.Context, options ...ReadSubjectsOption) <-chan ReadSubjectsResult {
	results := make(chan ReadSubjectsResult, 1)

	go func() {
		defer close(results)

		requestBody := readSubjectsRequestBody{
			BaseSubject: "/",
		}
		for _, option := range options {
			if err := option.apply(&requestBody); err != nil {
				results <- newReadSubjectsError(
					customErrors.NewInvalidParameterError(option.name, err.Error()),
				)
				return
			}
		}

		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			results <- newReadSubjectsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		requestFactory := httputil.NewRequestFactory(client.configuration)
		executeRequest, err := requestFactory.Create(
			http.MethodPost,
			"api/read-subjects",
			bytes.NewReader(requestBodyAsJSON),
		)
		if err != nil {
			results <- newReadSubjectsError(
				customErrors.NewInternalError(err),
			)
			return
		}

		response, err := executeRequest(ctx)
		if err != nil {
			results <- newReadSubjectsError(err)
			return
		}
		defer response.Body.Close()

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readSubjectsResponseItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				if contextutils.IsContextTerminationError(err) {
					results <- newReadSubjectsError(err)
					return
				}

				results <- newReadSubjectsError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: %s", err.Error())),
				)
				return
			}

			switch data.Type {
			case "error":
				var serverError streamError
				if err := json.Unmarshal(data.Payload, &serverError); err != nil {
					results <- newReadSubjectsError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream error encountered: %s", err.Error())),
					)
					return
				}

				results <- newReadSubjectsError(customErrors.NewServerError(serverError.Error))
			case "subject":
				var subject Subject
				if err := json.Unmarshal(data.Payload, &subject); err != nil {
					results <- newReadSubjectsError(
						customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%s' (trying to unmarshal '%+v')", err.Error(), data)),
					)
					return
				}

				results <- newSubject(subject)
			default:
				results <- newReadSubjectsError(
					customErrors.NewServerError(fmt.Sprintf("unsupported stream item encountered: '%+v' does not have a recognized type", data)),
				)
				return
			}
		}
	}()

	return results
}
