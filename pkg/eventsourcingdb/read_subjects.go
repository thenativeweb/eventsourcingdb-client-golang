package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/httpUtil"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"net/http"
	"net/url"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/result"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
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

func (client *Client) ReadSubjects(ctx context.Context, options ...ReadSubjectOption) <-chan ReadSubjectsResult {
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

		routeURL := client.configuration.baseURL + "/api/read-subjects"
		if _, err := url.Parse(routeURL); err != nil {
			results <- newReadSubjectsError(
				customErrors.NewInvalidParameterError(
					"client.configuration.baseURL",
					err.Error(),
				),
			)
			return
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		request, err := http.NewRequest("POST", routeURL, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			results <- newReadSubjectsError(
				customErrors.NewInternalError(err),
			)
			return
		}
		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response
		err = retry.WithBackoff(ctx, client.configuration.maxTries, func() error {
			response, err = httpClient.Do(request)

			if httpUtil.IsServerError(response) {
				return fmt.Errorf("server error: %s", response.Status)
			}

			return err
		})
		if err != nil {
			if customErrors.IsContextCanceledError(err) {
				results <- newReadSubjectsError(err)
				return
			}

			results <- newReadSubjectsError(
				customErrors.NewServerError(err.Error()),
			)
			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			results <- newReadSubjectsError(
				customErrors.NewClientError(err.Error()),
			)
			return
		}

		if httpUtil.IsClientError(response) {
			results <- newReadSubjectsError(
				customErrors.NewClientError(response.Status),
			)
			return
		}
		if response.StatusCode != http.StatusOK {
			results <- newReadSubjectsError(
				customErrors.NewServerError(fmt.Sprintf("unexpected response status: %s", response.Status)),
			)
			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readSubjectsResponseItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				if customErrors.IsContextCanceledError(err) {
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
