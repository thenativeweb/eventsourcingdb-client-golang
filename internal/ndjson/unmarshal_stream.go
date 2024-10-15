package ndjson

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
)

func UnmarshalStream[TData any](ctx context.Context, reader io.Reader) iter.Seq2[TData, error] {
	return func(yield func(TData, error) bool) {
		var zeroResult TData
		scanner := bufio.NewScanner(reader)
		lineChannel := make(chan string)

		go func() {
			defer close(lineChannel)
			for scanner.Scan() {
				if err := scanner.Err(); err != nil {
					yield(zeroResult, err)

					return
				}

				lineChannel <- scanner.Text()
			}
		}()

		for {
			select {
			case <-ctx.Done():
				yield(zeroResult, ctx.Err())
				return
			case currentLine, ok := <-lineChannel:
				if !ok {
					return
				}

				var data TData
				err := json.Unmarshal([]byte(currentLine), &data)
				if err != nil {
					yield(zeroResult, fmt.Errorf("cannot unmarshal '%s': %w", currentLine, err))
					break
				}

				if !yield(data, nil) {
					return
				}
			}
		}
	}
}
