package ndjson

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/result"
	"io"
)

type UnmarshalStreamResult[TData any] struct {
	result.Result[TData]
}

func newError[TData any](err error) UnmarshalStreamResult[TData] {
	return UnmarshalStreamResult[TData]{
		result.NewResultWithError[TData](err),
	}
}

func newData[TData any](data TData) UnmarshalStreamResult[TData] {
	return UnmarshalStreamResult[TData]{
		result.NewResultWithData[TData](data),
	}
}

func UnmarshalStream[TData any](context context.Context, reader io.Reader) <-chan UnmarshalStreamResult[TData] {
	scanner := bufio.NewScanner(reader)
	resultChannel := make(chan UnmarshalStreamResult[TData], 1)

	go func() {
		defer close(resultChannel)

		for scanner.Scan() {
			select {
			case <-context.Done():
				resultChannel <- newError[TData](errors.New("context cancelled"))

				return
			default:
				if err := scanner.Err(); err != nil {
					resultChannel <- newError[TData](err)

					return
				}

				var data TData

				currentLine := scanner.Text()
				if err := json.Unmarshal([]byte(currentLine), &data); err != nil {
					resultChannel <- newError[TData](err)

					return
				}

				resultChannel <- newData(data)
			}
		}
	}()

	return resultChannel
}
