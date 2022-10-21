package ndjson

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/error_container"
	"io"
)

type UnmarshalStreamResult[TData any] struct {
	error_container.ErrorContainer
	Data TData
}

func newUnmarshalStreamResult[TData any](data TData, err error) UnmarshalStreamResult[TData] {
	return UnmarshalStreamResult[TData]{
		ErrorContainer: error_container.NewErrorContainer(err),
		Data:           data,
	}
}

func UnmarshalStream[TData any](context context.Context, reader io.Reader) <-chan UnmarshalStreamResult[TData] {
	scanner := bufio.NewScanner(reader)
	resultChannel := make(chan UnmarshalStreamResult[TData], 1)
	var nilValue TData

	go func() {
		defer close(resultChannel)

		for scanner.Scan() {
			select {
			case <-context.Done():
				resultChannel <- newUnmarshalStreamResult[TData](nilValue, errors.New("context cancelled"))

				return
			default:
				if err := scanner.Err(); err != nil {
					resultChannel <- newUnmarshalStreamResult[TData](nilValue, err)

					return
				}

				var data TData

				currentLine := scanner.Text()
				if err := json.Unmarshal([]byte(currentLine), &data); err != nil {
					resultChannel <- newUnmarshalStreamResult[TData](nilValue, err)

					return
				}

				resultChannel <- newUnmarshalStreamResult(data, nil)
			}
		}
	}()

	return resultChannel
}
