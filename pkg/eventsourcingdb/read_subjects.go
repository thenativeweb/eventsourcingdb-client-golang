package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/result"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/retry"
	"net/http"
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
		result.NewResultWithData[string](subject.Subject),
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
	resultChannel := make(chan ReadSubjectsResult, 1)

	go func() {
		defer close(resultChannel)

		requestBody := readSubjectsRequestBody{
			BaseSubject: "/",
		}
		for _, applyOption := range options {
			if err := applyOption(&requestBody); err != nil {
				resultChannel <- newReadSubjectsError(err)

				return
			}
		}

		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			resultChannel <- newReadSubjectsError(err)

			return
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		url := client.configuration.baseURL + "/api/read-subjects"
		request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			resultChannel <- newReadSubjectsError(err)

			return
		}

		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response

		err = retry.WithBackoff(ctx, client.configuration.maxTries, func() error {
			response, err = httpClient.Do(request)

			return err
		})
		if err != nil {
			resultChannel <- newReadSubjectsError(err)

			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			resultChannel <- newReadSubjectsError(err)

			return
		}

		if response.StatusCode != http.StatusOK {
			resultChannel <- newReadSubjectsError(errors.New(fmt.Sprintf("failed to write events: %s", response.Status)))

			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readSubjectsResponseItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				resultChannel <- newReadSubjectsError(err)

				return
			}

			switch data.Type {
			case "subject":
				var subject Subject
				if err := json.Unmarshal(data.Payload, &subject); err != nil {
					resultChannel <- newReadSubjectsError(err)

					return
				}

				resultChannel <- newSubject(subject)
			default:
				resultChannel <- newReadSubjectsError(errors.New(fmt.Sprintf("unexpected stream item %+v", data)))

				return
			}
		}
	}()

	return resultChannel
}
