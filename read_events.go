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

type ReadEventsOptionsOrder string

const (
	oldestFirst ReadEventsOptionsOrder = "oldest-first"
	newestFirst ReadEventsOptionsOrder = "newest-first"
)

type ReadEventsOptions struct {
	WithSubStreams *bool
	Order          *ReadEventsOptionsOrder
	EventNames     *[]string
	LowerBoundID   *int64
	UpperBoundID   *int64
	FromEventName  *string
}

type readEventsRequest struct {
	StreamName string            `json:"streamName,omitempty"`
	Options    ReadEventsOptions `json:"options,omitempty"`
}

type eventStreamItem struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ReadEventsResult struct {
	result.Result[StoreItem]
}

func newError(err error) ReadEventsResult {
	return ReadEventsResult{
		result.NewResultWithError[StoreItem](err),
	}
}

func newStoreItem(item StoreItem) ReadEventsResult {
	return ReadEventsResult{
		result.NewResultWithData[StoreItem](item),
	}
}

func (client Client) ReadEvents(ctx context.Context, streamName string, options ReadEventsOptions) <-chan ReadEventsResult {
	resultChannel := make(chan ReadEventsResult, 1)

	go func() {
		defer close(resultChannel)
		requestBody := readEventsRequest{
			StreamName: streamName,
			Options:    options,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			resultChannel <- newError(err)

			return
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		url := client.configuration.baseURL + "/api/read-events"
		request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			resultChannel <- newError(err)

			return
		}

		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response

		err = retry.WithBackoff(func() error {
			response, err = httpClient.Do(request)

			return err
		}, client.configuration.maxTries, ctx)
		if err != nil {
			resultChannel <- newError(err)

			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			resultChannel <- newError(err)

			return
		}

		if response.StatusCode != http.StatusOK {
			resultChannel <- newError(errors.New(fmt.Sprintf("failed to read events: %s", response.Status)))

			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[eventStreamItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				resultChannel <- newError(err)

				return
			}

			switch data.Type {
			case "item":
				var storeItem StoreItem
				if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
					resultChannel <- newError(err)

					return
				}

				resultChannel <- newStoreItem(storeItem)
			default:
				resultChannel <- newError(errors.New(fmt.Sprintf("unexpected stream item %+v", data)))

				return
			}
		}
	}()

	return resultChannel
}
