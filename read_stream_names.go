package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/error_container"
	"github.com/thenativeweb/eventsourcingdb-client-golang/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
	"net/http"
)

type ReadStreamNamesResult struct {
	error_container.ErrorContainer
	StreamName *string
}

func NewReadStreamNamesResult(streamName *string, err error) ReadStreamNamesResult {
	return ReadStreamNamesResult{
		ErrorContainer: error_container.NewErrorContainer(err),
		StreamName:     streamName,
	}
}

type readStreamNamesRequestBody struct {
	BaseStreamName string `json:"baseStreamName"`
}

type readStreamNamesResponseItemPayload struct {
	StreamName string `json:"streamName"`
}

type readStreamNamesResponseItem struct {
	Type    string                             `json:"type"`
	Payload readStreamNamesResponseItemPayload `json:"payload"`
}

func (client *Client) ReadStreamNamesWithBaseStreamName(ctx context.Context, baseStreamName string) <-chan ReadStreamNamesResult {
	resultChannel := make(chan ReadStreamNamesResult, 1)

	go func() {
		defer close(resultChannel)

		requestBody := readStreamNamesRequestBody{
			BaseStreamName: baseStreamName,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			resultChannel <- NewReadStreamNamesResult(nil, err)

			return
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		url := client.configuration.baseURL + "/api/read-stream-names"
		request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			resultChannel <- NewReadStreamNamesResult(nil, err)

			return
		}

		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response

		err = retry.WithBackoff(func() error {
			response, err = httpClient.Do(request)

			return err
		}, client.configuration.maxTries, ctx)
		if err != nil {
			resultChannel <- NewReadStreamNamesResult(nil, err)

			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			resultChannel <- NewReadStreamNamesResult(nil, err)

			return
		}

		if response.StatusCode != http.StatusOK {
			resultChannel <- NewReadStreamNamesResult(nil, errors.New(fmt.Sprintf("failed to write events: %s", response.Status)))

			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readStreamNamesResponseItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			if unmarshalResult.IsError() {
				resultChannel <- NewReadStreamNamesResult(nil, unmarshalResult.Error)

				return
			}

			resultChannel <- NewReadStreamNamesResult(&unmarshalResult.Data.Payload.StreamName, nil)
		}
	}()

	return resultChannel
}

func (client *Client) ReadStreamNames(context context.Context) <-chan ReadStreamNamesResult {
	return client.ReadStreamNamesWithBaseStreamName(context, "/")
}
