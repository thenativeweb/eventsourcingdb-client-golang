package eventsourcingdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/authorization"
	"github.com/thenativeweb/eventsourcingdb-client-golang/ndjson"
	"github.com/thenativeweb/eventsourcingdb-client-golang/result"
	"github.com/thenativeweb/eventsourcingdb-client-golang/retry"
)

type observeEventsRequest struct {
	StreamName string               `json:"streamName,omitempty"`
	Options    ObserveEventsOptions `json:"options,omitempty"`
}

type ObserveEventsResult struct {
	result.Result[StoreItem]
}

func newObserveEventsError(err error) ObserveEventsResult {
	return ObserveEventsResult{
		result.NewResultWithError[StoreItem](err),
	}
}

func newObserveEventsValue(item StoreItem) ObserveEventsResult {
	return ObserveEventsResult{
		result.NewResultWithData[StoreItem](item),
	}
}

func (client *Client) ObserveEventsWithOptions(ctx context.Context, streamName string, options ObserveEventsOptions) <-chan ObserveEventsResult {
	resultChannel := make(chan ObserveEventsResult, 1)

	go func() {
		defer close(resultChannel)
		requestBody := observeEventsRequest{
			StreamName: streamName,
			Options:    options,
		}
		requestBodyAsJSON, err := json.Marshal(requestBody)
		if err != nil {
			resultChannel <- newObserveEventsError(err)

			return
		}

		httpClient := &http.Client{
			Timeout: client.configuration.timeout,
		}
		url := client.configuration.baseURL + "/api/observe-events"
		request, err := http.NewRequest("POST", url, bytes.NewReader(requestBodyAsJSON))
		if err != nil {
			resultChannel <- newObserveEventsError(err)

			return
		}

		authorization.AddAccessToken(request, client.configuration.accessToken)

		var response *http.Response

		err = retry.WithBackoff(ctx, client.configuration.maxTries, func() error {
			response, err = httpClient.Do(request)

			return err
		})
		if err != nil {
			resultChannel <- newObserveEventsError(err)

			return
		}
		defer response.Body.Close()

		err = client.validateProtocolVersion(response)
		if err != nil {
			resultChannel <- newObserveEventsError(err)

			return
		}

		if response.StatusCode != http.StatusOK {
			resultChannel <- newObserveEventsError(errors.New(fmt.Sprintf("failed to observe events: %s", response.Status)))

			return
		}

		unmarshalContext, cancelUnmarshalling := context.WithCancel(ctx)
		defer cancelUnmarshalling()

		unmarshalResults := ndjson.UnmarshalStream[eventStreamItem](unmarshalContext, response.Body)
		for unmarshalResult := range unmarshalResults {
			data, err := unmarshalResult.GetData()
			if err != nil {
				resultChannel <- newObserveEventsError(err)

				return
			}

			switch data.Type {
			case "heartbeat":
				continue
			case "item":
				var storeItem StoreItem
				if err := json.Unmarshal(data.Payload, &storeItem); err != nil {
					resultChannel <- newObserveEventsError(err)

					return
				}

				resultChannel <- newObserveEventsValue(storeItem)
			default:
				resultChannel <- newObserveEventsError(errors.New(fmt.Sprintf("unexpected stream item %+v", data)))

				return
			}
		}
	}()

	return resultChannel
}

func (client *Client) ObserveEvents(ctx context.Context, streamName string, recursive bool) <-chan ObserveEventsResult {
	return client.ObserveEventsWithOptions(ctx, streamName, NewObserveEventsOptions(recursive))
}
