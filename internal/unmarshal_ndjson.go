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
		// By default, bufio.Scanner has a maximum token size of 64KB.
		// Since the event payload alone can reach 64KB, we increase the scanner's
		// buffer size to accommodate the payload plus its metadata.
		buf := make([]byte, 0, 100*1024)
		scanner.Buffer(buf, 100*1024)

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
