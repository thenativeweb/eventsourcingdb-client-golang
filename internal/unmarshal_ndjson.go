package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"iter"
)

type Line struct {
	Type    string
	Payload json.RawMessage
}

func UnmarshalNDJSON(ctx context.Context, r io.Reader) iter.Seq2[Line, error] {
	return func(yield func(Line, error) bool) {
		scanner := bufio.NewScanner(r)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				yield(Line{}, ctx.Err())
				return
			default:
				// From a technical point of view, this default block
				// is not necessary. Actually, the entire select is
				// only there to avoid blocking, and a select with just
				// a single case would be sufficient, as it does not
				// block anyway. However, this is a common pattern in
				// Go, and it is used here to make the code more
				// readable.
			}

			var line Line
			jsonString := scanner.Text()

			err := json.Unmarshal([]byte(jsonString), &line)
			if err != nil {
				if !yield(Line{}, err) {
					return
				}
				continue
			}

			if !yield(line, nil) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(Line{}, err)
		}
	}
}
