package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/result"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
	"net/http"
)

type StreamName struct {
	StreamName string `json:"streamName"`
}

type ReadStreamNamesResult struct {
	result.Result[StreamName]
}

func newReadStreamNamesError(err error) ReadStreamNamesResult {
	return ReadStreamNamesResult{
		result.NewResultWithError[StreamName](err),
	}
}

func newStreamName(streamName StreamName) ReadStreamNamesResult {
	return ReadStreamNamesResult{
		result.NewResultWithData[StreamName](streamName),
	}
}

type readStreamNamesRequestBody struct {
	BaseStreamName string `json:"baseStreamName"`
}

type readStreamNamesResponseItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
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
			resultChannel <- newReadStreamNamesError(err)

			return
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		url := client.configuration.baseURL + "/api/read-stream-names"
		request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			resultChannel <- newReadStreamNamesError(err)

			return
		}

		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response

		err = retry.WithBackoff(func() error {
			response, err = httpClient.Do(request)

			return err
		}, client.configuration.maxTries, ctx)
		if err != nil {
			resultChannel <- newReadStreamNamesError(err)

			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			resultChannel <- newReadStreamNamesError(err)

			return
		}

		if response.StatusCode != http.StatusOK {
			resultChannel <- newReadStreamNamesError(errors.New(fmt.Sprintf("failed to write events: %s", response.Status)))

			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[readStreamNamesResponseItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				resultChannel <- newReadStreamNamesError(err)

				return
			}

			switch data.Type {
			case "streamName":
				var streamName StreamName
				if err := json.Unmarshal(data.Payload, &streamName); err != nil {
					resultChannel <- newReadStreamNamesError(err)

					return
				}

				resultChannel <- newStreamName(streamName)
			default:
				resultChannel <- newReadStreamNamesError(errors.New(fmt.Sprintf("unexpected stream item %+v", data)))

				return
			}
		}
	}()

	return resultChannel
}

func (client *Client) ReadStreamNames(ctx context.Context) <-chan ReadStreamNamesResult {
	return client.ReadStreamNamesWithBaseStreamName(ctx, "/")
}
