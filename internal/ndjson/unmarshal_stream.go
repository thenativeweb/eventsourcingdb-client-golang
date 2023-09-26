package ndjson

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/thenativeweb/goutils/v2/coreutils/result"
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
		result.NewResultWithData(data),
	}
}

func UnmarshalStream[TData any](ctx context.Context, reader io.Reader) <-chan UnmarshalStreamResult[TData] {
	scanner := bufio.NewScanner(reader)
	resultChannel := make(chan UnmarshalStreamResult[TData], 1)
	lineChannel := make(chan string)

	go func() {
		defer close(lineChannel)
		for scanner.Scan() {
			if err := scanner.Err(); err != nil {
				resultChannel <- newError[TData](err)

				return
			}

			lineChannel <- scanner.Text()
		}
	}()

	go func() {
		defer close(resultChannel)

	LineLoop:
		for {
			select {
			case <-ctx.Done():
				resultChannel <- newError[TData](ctx.Err())
			case currentLine, ok := <-lineChannel:
				if !ok {
					break LineLoop
				}

				var data TData
				if err := json.Unmarshal([]byte(currentLine), &data); err != nil {
					resultChannel <- newError[TData](fmt.Errorf("cannot unmarshal '%s': %w", currentLine, err))
					break
				}

				resultChannel <- newData(data)
			}
		}
	}()

	return resultChannel
}
